package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunVersionPrintsVersion(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"version"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	if strings.TrimSpace(out.String()) != version {
		t.Fatalf("stdout = %q, want %q", out.String(), version)
	}
}

func TestRunNoArgsIsUsageError(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run(nil, &out, &errOut); code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "usage:") {
		t.Fatalf("stderr = %q, want a usage line", errOut.String())
	}
}
