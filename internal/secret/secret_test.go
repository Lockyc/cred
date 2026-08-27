package secret

import (
	"errors"
	"strings"
	"testing"
)

func TestTrimStripsTrailingNewlineAndCR(t *testing.T) {
	cases := map[string]string{
		"abc":     "abc",
		"abc\n":   "abc",
		"abc\r\n": "abc",
		"abc\n\n": "abc",
		"  abc  ": "  abc  ", // interior/leading spaces are preserved
		"a b\n":   "a b",
	}
	for in, want := range cases {
		if got := Trim(in); got != want {
			t.Errorf("Trim(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFromCommandReturnsStdoutTrimmed(t *testing.T) {
	got, err := FromCommand("printf 'tok_abc\\n'")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != "tok_abc" {
		t.Fatalf("got %q, want %q", got, "tok_abc")
	}
}

func TestFromCommandEmptyStdoutIsErrEmpty(t *testing.T) {
	_, err := FromCommand("true")
	if !errors.Is(err, ErrEmpty) {
		t.Fatalf("err = %v, want ErrEmpty", err)
	}
}

func TestFromCommandPropagatesFailure(t *testing.T) {
	_, err := FromCommand("exit 3")
	if err == nil {
		t.Fatal("want an error when the command fails")
	}
	if errors.Is(err, ErrEmpty) {
		t.Fatal("a failing command must not be reported as an empty value")
	}
}

func TestFromCommandFailureIncludesStderr(t *testing.T) {
	_, err := FromCommand("printf 'no such item\\n' >&2; exit 1")
	if err == nil {
		t.Fatal("want an error when the command fails")
	}
	if !strings.Contains(err.Error(), "no such item") {
		t.Fatalf("err = %q, want it to contain the child's stderr text %q", err.Error(), "no such item")
	}
}

func TestFromCommandFailureWithEmptyStderrIsClean(t *testing.T) {
	_, err := FromCommand("exit 3")
	if err == nil {
		t.Fatal("want an error when the command fails")
	}
	msg := err.Error()
	if strings.Contains(msg, "\"\"") {
		t.Fatalf("err = %q, must not contain empty quotes when stderr is empty", msg)
	}
	if strings.HasSuffix(strings.TrimSpace(msg), ":") {
		t.Fatalf("err = %q, must not have a dangling separator when stderr is empty", msg)
	}
}
