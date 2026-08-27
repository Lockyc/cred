// Package secret acquires a credential value. Every path here is careful about
// one thing: the value must never become a process argument, so it is either
// typed at a hidden prompt or read from another command's stdout.
package secret

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/term"
)

// ErrEmpty means the source produced nothing. Never write an empty credential:
// it is always a mis-paste or a mis-typed command, and an empty file looks
// exactly like a valid one to whatever reads it next.
var ErrEmpty = errors.New("empty value")

// Trim removes trailing newlines and carriage returns and nothing else. A value
// from `cat file` or a command's stdout almost always carries a trailing
// newline that is not part of the credential; a CR arrives from a CRLF paste.
// Leading and interior whitespace is preserved — it may be significant.
func Trim(raw string) string {
	return strings.TrimRight(raw, "\r\n")
}

// FromCommand runs cmd through sh -c and returns its trimmed stdout. The value
// reaches this process through a pipe, never through argv.
func FromCommand(cmd string) (string, error) {
	out, err := exec.Command("sh", "-c", cmd).Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if stderr := strings.TrimSpace(string(exitErr.Stderr)); stderr != "" {
				return "", fmt.Errorf("--value-from command failed: %w: %s", err, stderr)
			}
		}
		return "", fmt.Errorf("--value-from command failed: %w", err)
	}
	v := Trim(string(out))
	if v == "" {
		return "", ErrEmpty
	}
	return v, nil
}

// FromTTY prompts on the controlling terminal with echo disabled. It reads from
// /dev/tty rather than stdin so it still works when stdin is a pipe.
func FromTTY(prompt string) (string, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return "", fmt.Errorf("cred needs a terminal to prompt; use --value-from '<command>' instead: %w", err)
	}
	defer tty.Close()

	fmt.Fprintf(tty, "%s: ", prompt)
	raw, err := term.ReadPassword(int(tty.Fd()))
	fmt.Fprintln(tty)
	if err != nil {
		return "", fmt.Errorf("reading from terminal: %w", err)
	}
	v := Trim(string(raw))
	if v == "" {
		return "", ErrEmpty
	}
	return v, nil
}
