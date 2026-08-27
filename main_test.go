package main

import (
	"bytes"
	"os"
	"path/filepath"
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

func TestSetWritesFileAndPrintsReceipt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key")
	var out, errOut bytes.Buffer
	code := run([]string{"set", path, "--value-from", "printf 'cal_live_abc'"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut.String())
	}
	for _, want := range []string{"cred: OK", path, "600", "12", "Paste this block back"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("receipt missing %q:\n%s", want, out.String())
		}
	}
}

func TestSetRefusesOnPrefixMismatchAndWritesNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key")
	var out, errOut bytes.Buffer
	code := run([]string{"set", path, "--value-from", "printf 'wrong'", "--expect-prefix", "cal_live_"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("file was written despite the prefix mismatch")
	}
	if !strings.Contains(errOut.String(), "nothing written") {
		t.Fatalf("stderr = %q", errOut.String())
	}
}

func TestSetNeverPrintsTheValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key")
	var out, errOut bytes.Buffer
	run([]string{"set", path, "--value-from", "printf 'cal_live_TOPSECRET'"}, &out, &errOut)
	if strings.Contains(out.String()+errOut.String(), "TOPSECRET") {
		t.Fatalf("the value leaked into output:\n%s\n%s", out.String(), errOut.String())
	}
}

func TestSetEnvKeyEndToEnd(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("APP_ENV=local\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code := run([]string{"set", path, "--name", "API_KEY", "--value-from", "printf 'sk_live_x'"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut.String())
	}
	b, _ := os.ReadFile(path)
	if string(b) != "APP_ENV=local\nAPI_KEY=sk_live_x\n" {
		t.Fatalf("env file =\n%s", string(b))
	}
	if !strings.Contains(out.String(), "API_KEY") {
		t.Fatalf("receipt did not name the key:\n%s", out.String())
	}
}
