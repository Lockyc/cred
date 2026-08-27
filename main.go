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

// reportMissing prints the curated "credential absent" report that show and
// rm agree on. It goes on stdout, not stderr: MISSING is an expected outcome
// cred is built to report cleanly — the human's next move is usually
// `cred set` — not a runtime failure, which stays on stderr.
func reportMissing(stdout io.Writer, path string) {
	fmt.Fprintf(stdout, "cred: MISSING — %s does not exist\n", path)
}

// refuseNonRegular reports (and prints) whether path is something other than
// a regular file — a directory, a symlink, a device — so set, show and rm
// all refuse in the same voice instead of a raw OS error surfacing from
// whatever they'd otherwise attempt (a rename, a read, an unlink). fi must
// come from Lstat, not Stat, so a symlink is judged by its own type rather
// than resolved to whatever it points at.
func refuseNonRegular(stderr io.Writer, op, path string, fi os.FileInfo) bool {
	if fi.Mode().IsRegular() {
		return false
	}
	fmt.Fprintf(stderr, "cred: %s is not a regular file — refusing to %s\n", path, op)
	return true
}

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
	valueFrom := fs.String("value-from", "", "run this command and use its stdout as the value — the command "+
		"must FETCH the value (e.g. 'op read op://vault/item/field'), never contain it "+
		"(e.g. 'echo sk-live-...'), or the value ends up in this process's own argv")

	path, ok, code := parsePathCommand("set", "destination path", fs, args, stdout, stderr)
	if !ok {
		return code
	}

	mode, err := strconv.ParseUint(*modeStr, 8, 32)
	if err != nil {
		fmt.Fprintf(stderr, "cred: --mode %q is not an octal mode\n", *modeStr)
		return 2
	}

	// Checked up front, before prompting: a directory or symlink destination
	// is refused the same way show and rm refuse one, rather than reaching
	// os.Rename inside writeFileAtomic and surfacing a raw rename error —
	// and rather than prompting a human for a secret that can't be written
	// anyway. A path that doesn't exist yet is fine (Lstat's error is
	// ignored) — that's the normal create case.
	if fi, err := os.Lstat(path); err == nil {
		if refuseNonRegular(stderr, "write", path, fi) {
			return 1
		}
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

	path, ok, code := parsePathCommand("show", "path", fs, args, stdout, stderr)
	if !ok {
		return code
	}

	// Lstat, not Stat: a symlink is judged by its own type (see
	// refuseNonRegular) rather than resolved to whatever it points at.
	fi, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			reportMissing(stdout, path)
			return 1
		}
		// Any other stat error (EACCES on a parent directory, ELOOP, ENOTDIR,
		// ...) is not "the credential is absent" — reporting it as MISSING
		// would send someone to `cred set` and overwrite a live credential
		// that was merely unreadable.
		fmt.Fprintf(stderr, "cred: %v\n", err)
		return 1
	}
	if refuseNonRegular(stderr, "read", path, fi) {
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
		Path: path, Key: *key, Mode: modeString(fi),
		Bytes: len(value), Fingerprint: receipt.Fingerprint(value),
		Modified: fi.ModTime().Format("2006-01-02 15:04:05"),
	}
	fmt.Fprint(stdout, r.RenderShow())
	return 0
}

func runRm(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("rm", flag.ContinueOnError)
	key := fs.String("name", "", "remove this KEY from a .env file instead of deleting the file")

	path, ok, code := parsePathCommand("rm", "path", fs, args, stdout, stderr)
	if !ok {
		return code
	}

	// Checked once, up front, ahead of both the --name and whole-file paths
	// below — the presence check the invariant comment above `RemoveEnvKey`
	// gates on, and the same MISSING/non-regular report show uses. Lstat,
	// not Stat: a symlink is judged by its own type (see refuseNonRegular),
	// which is what stops rm from unlinking a symlink while leaving the
	// credential it points at untouched.
	fi, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			reportMissing(stdout, path)
			return 1
		}
		fmt.Fprintf(stderr, "cred: %v\n", err)
		return 1
	}
	if refuseNonRegular(stderr, "remove", path, fi) {
		return 1
	}

	if *key != "" {
		present, err := store.HasEnvKey(path, *key)
		if err != nil {
			fmt.Fprintf(stderr, "cred: %v\n", err)
			return 1
		}
		// RemoveEnvKey is a documented no-op when the key is absent, but it
		// still rewrites the file (writeFileAtomic runs regardless) — a new
		// inode, a bumped mtime, normalised line endings — even though
		// nothing changed. The presence check must gate the call to
		// RemoveEnvKey, before that write happens.
		if !present {
			fmt.Fprintf(stderr, "cred: key %s is not set in %s — nothing removed\n", *key, path)
			return 1
		}
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

// parsePathCommand runs the prologue set, show and rm each need before their
// own logic: split flags from the destination path (so the path may appear
// before, after, or between flags), parse the flags, reject a missing or
// extra positional argument, then expand a leading ~/. name and pathDesc
// carry the only piece that differs between callers — the command noun and
// the phrase describing what's missing in the usage-error message.
func parsePathCommand(name, pathDesc string, fs *flag.FlagSet, args []string, stdout, stderr io.Writer) (path string, ok bool, code int) {
	flagArgs, rawPath, extra := splitPathArgs(args)
	if parsed, c := parseFlags(fs, flagArgs, stdout, stderr); !parsed {
		return "", false, c
	}
	if rawPath == "" || len(extra) > 0 {
		fmt.Fprintf(stderr, "cred: %s needs exactly one %s\n\n%s", name, pathDesc, usage)
		return "", false, 2
	}
	path, err := expandTilde(rawPath)
	if err != nil {
		fmt.Fprintf(stderr, "cred: %v\n", err)
		// A ~user form is a malformed argument — the same class of mistake
		// as a bad --mode — so it's a usage error (2). An os.UserHomeDir
		// failure sharing this call is a genuine runtime error and stays 1;
		// errUnsupportedTildeUser is what tells the two apart.
		if errors.Is(err, errUnsupportedTildeUser) {
			return "", false, 2
		}
		return "", false, 1
	}
	return path, true, 0
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

// errUnsupportedTildeUser marks the "~user" rejection so parsePathCommand can
// tell it apart from a genuine os.UserHomeDir failure — the same expandTilde
// error return covers both, but only the latter is a runtime error.
var errUnsupportedTildeUser = errors.New("only ~/ is supported, not another user's home directory")

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
		return "", fmt.Errorf("%q: %w", p, errUnsupportedTildeUser)
	}
	return p, nil
}

func modeOf(path string) string {
	fi, err := os.Stat(path)
	if err != nil {
		return "?"
	}
	return modeString(fi)
}

// modeString renders a FileInfo's permission bits the way the receipt shows
// them. Shared by modeOf (which stats path itself) and runShow (which
// already holds the FileInfo from its own os.Stat), so the format lives in
// exactly one place.
func modeString(fi os.FileInfo) string {
	return fmt.Sprintf("%o", fi.Mode().Perm())
}
