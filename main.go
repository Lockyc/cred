package main

import (
	"bytes"
	"errors"
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
	case "show":
		return runShow(args[1:], stdout, stderr)
	case "rm":
		return runRm(args[1:], stdout, stderr)
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
	key := fs.String("name", "", "set this KEY inside a .env file instead of writing the whole file")
	expect := fs.String("expect-prefix", "", "refuse the value unless it starts with this")
	modeStr := fs.String("mode", "600", "octal file mode")
	valueFrom := fs.String("value-from", "", "run this command and use its stdout as the value")

	flagArgs, path, extra := splitPathArgs(args)
	if ok, code := parseFlags(fs, flagArgs, stdout, stderr); !ok {
		return code
	}
	if path == "" || len(extra) > 0 {
		fmt.Fprintf(stderr, "cred: set needs exactly one destination path\n\n%s", usage)
		return 2
	}
	path, err := expandTilde(path)
	if err != nil {
		fmt.Fprintf(stderr, "cred: %v\n", err)
		return 1
	}

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

func runShow(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("show", flag.ContinueOnError)
	key := fs.String("name", "", "report on this KEY inside a .env file")

	flagArgs, rawPath, extra := splitPathArgs(args)
	if ok, code := parseFlags(fs, flagArgs, stdout, stderr); !ok {
		return code
	}
	if rawPath == "" || len(extra) > 0 {
		fmt.Fprintf(stderr, "cred: show needs exactly one path\n\n%s", usage)
		return 2
	}
	path, err := expandTilde(rawPath)
	if err != nil {
		fmt.Fprintf(stderr, "cred: %v\n", err)
		return 1
	}

	fi, err := os.Stat(path)
	if err != nil {
		fmt.Fprintf(stdout, "cred: MISSING — %s does not exist\n", path)
		return 1
	}
	var value string
	if *key != "" {
		value, err = store.GetEnvKey(path, *key)
	} else {
		value, err = store.ReadFile(path)
	}
	if err != nil {
		fmt.Fprintf(stderr, "cred: %v\n", err)
		return 1
	}
	r := receipt.Receipt{
		Path: path, Key: *key, Mode: fmt.Sprintf("%o", fi.Mode().Perm()),
		Bytes: len(value), Fingerprint: receipt.Fingerprint(value),
		Modified: fi.ModTime().Format("2006-01-02 15:04:05"),
	}
	fmt.Fprint(stdout, r.RenderShow())
	return 0
}

func runRm(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("rm", flag.ContinueOnError)
	key := fs.String("name", "", "remove this KEY from a .env file instead of deleting the file")

	flagArgs, rawPath, extra := splitPathArgs(args)
	if ok, code := parseFlags(fs, flagArgs, stdout, stderr); !ok {
		return code
	}
	if rawPath == "" || len(extra) > 0 {
		fmt.Fprintf(stderr, "cred: rm needs exactly one path\n\n%s", usage)
		return 2
	}
	path, err := expandTilde(rawPath)
	if err != nil {
		fmt.Fprintf(stderr, "cred: %v\n", err)
		return 1
	}

	if *key != "" {
		if err := store.RemoveEnvKey(path, *key); err != nil {
			fmt.Fprintf(stderr, "cred: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "cred: removed key %s from %s\n", *key, path)
		return 0
	}
	if err := os.Remove(path); err != nil {
		fmt.Fprintf(stderr, "cred: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "cred: removed %s\n", path)
	return 0
}

// splitPathArgs separates flags from the destination path so a command
// accepts the path before, after, or between flags — flag.FlagSet.Parse
// alone stops parsing at the first non-flag argument, which would otherwise
// leave flags unparsed whenever they don't all trail the path. Every flag
// set, show and rm define takes a value (none are boolean), so a "-name" or
// "--name" token consumes the following token as its value; -h/--help is the
// one exception, since flag.FlagSet handles those with no operand and
// self-contained "-name=value" tokens need nothing consumed either. A "--"
// terminator ends flag scanning; everything after it — including a token
// that starts with "-" — is positional. extra holds any positional beyond
// the first, for the caller to reject as a usage error.
func splitPathArgs(args []string) (flagArgs []string, path string, extra []string) {
	afterTerm := false
	havePath := false
	takePositional := func(a string) {
		if !havePath {
			path, havePath = a, true
		} else {
			extra = append(extra, a)
		}
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case afterTerm:
			takePositional(a)
		case a == "--":
			afterTerm = true
		case len(a) > 1 && a[0] == '-':
			flagArgs = append(flagArgs, a)
			name := strings.TrimLeft(a, "-")
			if name != "h" && name != "help" && !strings.Contains(name, "=") && i+1 < len(args) {
				i++
				flagArgs = append(flagArgs, args[i])
			}
		default:
			takePositional(a)
		}
	}
	return flagArgs, path, extra
}

// parseFlags runs fs.Parse against a buffer rather than directly against
// stderr, so a -h/--help result (flag.ErrHelp) can be redirected to stdout
// with exit 0 — the documented affordance ("Run 'cred <command> -h' for the
// options of one command") — instead of falling into the same
// stderr-and-usage-error-2 path as a real parse failure.
func parseFlags(fs *flag.FlagSet, args []string, stdout, stderr io.Writer) (ok bool, code int) {
	var buf bytes.Buffer
	fs.SetOutput(&buf)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			buf.WriteTo(stdout)
			return false, 0
		}
		buf.WriteTo(stderr)
		return false, 2
	}
	return true, 0
}

// expandTilde handles a leading ~/ that arrived quoted. An unquoted ~ is
// expanded by the shell, but a pasted path is often quoted and would
// otherwise create a literal "~" directory — so this must fail closed rather
// than fall through to a literal ~-prefixed path in any case: neither an
// os.UserHomeDir error nor a ~user form (which it does not resolve) is
// treated as "leave the path alone".
func expandTilde(p string) (string, error) {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expanding %q: %w", p, err)
		}
		return filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(p, "~"), "/")), nil
	}
	if strings.HasPrefix(p, "~") {
		return "", fmt.Errorf("%q: only ~/ is supported, not another user's home directory", p)
	}
	return p, nil
}

func modeOf(path string) string {
	fi, err := os.Stat(path)
	if err != nil {
		return "?"
	}
	return fmt.Sprintf("%o", fi.Mode().Perm())
}
