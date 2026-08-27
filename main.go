package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/lockyc/cred/internal/receipt"
	"github.com/lockyc/cred/internal/secret"
	"github.com/lockyc/cred/internal/store"
)

const usage = `usage: cred <command> [options]

  cred set  <path> [--name KEY] [--expect-prefix P] [--mode M] [--value-from CMD]
  cred show <path> [--name KEY]
  cred rm   <path> [--name KEY]
  cred version

Run 'cred <command> -h' for the options of one command.
`

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "set":
		return runSet(args[1:], stdout, stderr)
	case "version":
		fmt.Fprintln(stdout, version)
		return 0
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "cred: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}

func runSet(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("set", flag.ContinueOnError)
	fs.SetOutput(stderr)
	key := fs.String("name", "", "set this KEY inside a .env file instead of writing the whole file")
	expect := fs.String("expect-prefix", "", "refuse the value unless it starts with this")
	modeStr := fs.String("mode", "600", "octal file mode")
	valueFrom := fs.String("value-from", "", "run this command and use its stdout as the value")

	// The documented usage is `cred set <path> [options]`, path first — but
	// flag.FlagSet.Parse stops at the first non-flag argument, so handing it
	// args unmodified would read the path as the end of the flag list and
	// leave every flag after it unparsed. Pull the path out by hand when it
	// leads (the common case), so the flags that follow still reach Parse;
	// otherwise fall through to a plain positional after the flags.
	var path string
	rest := args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		path = args[0]
		rest = args[1:]
	}
	if err := fs.Parse(rest); err != nil {
		return 2
	}
	if path == "" {
		if fs.NArg() != 1 {
			fmt.Fprintf(stderr, "cred: set needs exactly one destination path\n\n%s", usage)
			return 2
		}
		path = fs.Arg(0)
	} else if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "cred: set needs exactly one destination path\n\n%s", usage)
		return 2
	}
	path = expandTilde(path)

	mode, err := strconv.ParseUint(*modeStr, 8, 32)
	if err != nil {
		fmt.Fprintf(stderr, "cred: --mode %q is not an octal mode\n", *modeStr)
		return 2
	}

	label := path
	if *key != "" {
		label = *key
	}
	var value string
	if *valueFrom != "" {
		value, err = secret.FromCommand(*valueFrom)
	} else {
		value, err = secret.FromTTY("Value for " + label)
	}
	if err != nil {
		fmt.Fprintf(stderr, "cred: %v — nothing written\n", err)
		return 1
	}
	if *expect != "" && !strings.HasPrefix(value, *expect) {
		// Refuse rather than write. A wrong-shaped paste caught here costs one
		// retry; caught later it looks like a broken API rather than a bad paste.
		fmt.Fprintf(stderr, "cred: value does not start with %q — nothing written.\nCheck you pasted the right credential.\n", *expect)
		return 1
	}

	if *key != "" {
		err = store.SetEnvKey(path, *key, value, os.FileMode(mode))
	} else {
		err = store.WriteFile(path, value, os.FileMode(mode))
	}
	if err != nil {
		fmt.Fprintf(stderr, "cred: %v\n", err)
		return 1
	}

	r := receipt.Receipt{
		Path: path, Key: *key, Bytes: len(value),
		Fingerprint: receipt.Fingerprint(value), Prefix: *expect,
		Mode: modeOf(path),
	}
	fmt.Fprint(stdout, r.RenderSet())
	return 0
}

// expandTilde handles a leading ~/ that arrived quoted. An unquoted ~ is
// expanded by the shell, but a pasted path is often quoted and would otherwise
// create a literal "~" directory.
func expandTilde(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(p, "~"), "/"))
		}
	}
	return p
}

func modeOf(path string) string {
	fi, err := os.Stat(path)
	if err != nil {
		return "?"
	}
	return fmt.Sprintf("%o", fi.Mode().Perm())
}
