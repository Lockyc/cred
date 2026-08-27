package secret

import (
	"errors"
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
