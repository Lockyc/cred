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

// ErrAborted means the user aborted the hidden prompt (Ctrl-C, or Ctrl-D on
// empty input). Nothing is written on this path.
var ErrAborted = errors.New("aborted")

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

// FromTTY prompts on the controlling terminal with each entered rune masked
// as ● (U+25CF), never the rune itself. It reads from /dev/tty rather than
// stdin so it still works when stdin is a pipe.
//
// A silent prompt — the previous behaviour, golang.org/x/term's
// ReadPassword — gives the operator no way to tell a registered keystroke
// from a dropped one: zero characters and forty look identical. The whole
// point of cred is giving the human confidence the credential landed, so
// going silent at the moment they enter it undercuts that. All the
// character-accounting logic (unit counting, backspace, abort, terminators,
// escape-sequence and invalid-UTF-8 handling) lives in keyState.feed
// (keyreader.go), a pure function with no terminal
// dependency — FromTTY itself needs a real terminal and so cannot be
// unit-tested; that's exactly why the logic doesn't live here.
func FromTTY(prompt string) (string, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return "", fmt.Errorf("cred needs a terminal to prompt; use --value-from '<command>' instead: %w", err)
	}
	defer tty.Close()

	fd := int(tty.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return "", fmt.Errorf("reading from terminal: %w", err)
	}
	// Restored on every exit path from here down, including abort and error
	// returns — a raw terminal left unrestored breaks every command typed
	// into it afterwards.
	defer term.Restore(fd, oldState)

	fmt.Fprintf(tty, "%s: ", prompt)

	k := newKeyState()
	buf := make([]byte, 256)
	var aborted bool
readLoop:
	for {
		n, err := tty.Read(buf)
		if n > 0 {
			out, done, abort := k.feed(buf[:n])
			if len(out) > 0 {
				tty.Write(out)
			}
			if abort {
				aborted = true
				break readLoop
			}
			if done {
				break readLoop
			}
		}
		if err != nil {
			// term.Restore hasn't run yet (it's deferred, not yet reached),
			// so the tty is still in raw mode here: MakeRaw clears OPOST, so
			// a bare \n moves down a line without a carriage return, leaving
			// the shell prompt and everything printed after indented under
			// the last mask. \r\n is required on every write to tty while
			// raw is in effect.
			fmt.Fprint(tty, "\r\n")
			return "", fmt.Errorf("reading from terminal: %w", err)
		}
	}
	fmt.Fprint(tty, "\r\n")

	if aborted {
		return "", ErrAborted
	}

	v := Trim(k.value())
	if v == "" {
		return "", ErrEmpty
	}
	return v, nil
}
