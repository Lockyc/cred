package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lockyc/cred/internal/store"
)

// row mirrors the private row format in internal/receipt/receipt.go
// (fmt.Sprintf("  %-12s %s", key, value)) so tests can assert the exact
// rendered line rather than a substring loose enough to match unrelated
// digits elsewhere in the receipt (the fingerprint, the temp path, ...).
func row(key, value string) string {
	return fmt.Sprintf("  %-12s %s", key, value)
}

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
	if !strings.Contains(out.String(), "cred: OK") || !strings.Contains(out.String(), "Paste this block back") {
		t.Fatalf("receipt missing the OK header or paste-back line:\n%s", out.String())
	}
	if !strings.Contains(out.String(), row("path", path)) {
		t.Fatalf("receipt missing path row %q:\n%s", row("path", path), out.String())
	}
	// "cal_live_abc" is exactly 12 bytes: the bytes row must read exactly
	// "12", not merely contain "12" (which the fingerprint or temp path
	// could also satisfy).
	if !strings.Contains(out.String(), row("bytes", "12")) {
		t.Fatalf("receipt missing exact bytes row %q:\n%s", row("bytes", "12"), out.String())
	}
	if !strings.Contains(out.String(), row("mode", "600")) {
		t.Fatalf("receipt missing exact mode row %q:\n%s", row("mode", "600"), out.String())
	}

	got, err := store.ReadFile(path)
	if err != nil {
		t.Fatalf("reading back the written file: %v", err)
	}
	if got != "cal_live_abc" {
		t.Fatalf("file contents = %q, want %q", got, "cal_live_abc")
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
	code := run([]string{"set", path, "--value-from", "printf 'cal_live_TOPSECRET'"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s) — a non-zero exit would make the leak check pass trivially", code, errOut.String())
	}
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

// --- set's path/flag extraction ---
//
// flag.FlagSet.Parse stops at the first non-flag argument, so `set` has to
// pull the destination path out by hand before handing the rest to Parse.
// These cover every shape that hand-rolled extraction has to get right.

func TestSetFlagsBeforePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key")
	var out, errOut bytes.Buffer
	code := run([]string{"set", "--mode", "644", path, "--value-from", "printf 'cal_live_abc'"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut.String())
	}
	got, err := store.ReadFile(path)
	if err != nil {
		t.Fatalf("reading back the written file: %v", err)
	}
	if got != "cal_live_abc" {
		t.Fatalf("file contents = %q, want %q", got, "cal_live_abc")
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o644 {
		t.Fatalf("mode = %o, want 644 (the --mode given before the path)", fi.Mode().Perm())
	}
}

func TestSetFlagsAfterPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key")
	var out, errOut bytes.Buffer
	code := run([]string{"set", path, "--mode", "640", "--value-from", "printf 'cal_live_abc'"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut.String())
	}
	got, err := store.ReadFile(path)
	if err != nil {
		t.Fatalf("reading back the written file: %v", err)
	}
	if got != "cal_live_abc" {
		t.Fatalf("file contents = %q, want %q", got, "cal_live_abc")
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o640 {
		t.Fatalf("mode = %o, want 640 (the --mode given after the path)", fi.Mode().Perm())
	}
}

func TestSetDashDashTerminatorAllowsPathStartingWithDash(t *testing.T) {
	dir := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldwd)

	var out, errOut bytes.Buffer
	code := run([]string{"set", "--value-from", "printf 'cal_live_abc'", "--", "-sekret"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut.String())
	}
	got, err := store.ReadFile(filepath.Join(dir, "-sekret"))
	if err != nil {
		t.Fatalf("reading back the written file: %v", err)
	}
	if got != "cal_live_abc" {
		t.Fatalf("file contents = %q, want %q", got, "cal_live_abc")
	}
}

func TestSetMissingPathIsUsageError(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"set", "--value-from", "printf 'cal_live_abc'"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (stdout: %s, stderr: %s)", code, out.String(), errOut.String())
	}
}

func TestSetTwoPositionalArgumentsIsUsageError(t *testing.T) {
	p1 := filepath.Join(t.TempDir(), "key1")
	p2 := filepath.Join(t.TempDir(), "key2")
	var out, errOut bytes.Buffer
	code := run([]string{"set", p1, p2}, &out, &errOut)
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (stdout: %s, stderr: %s)", code, out.String(), errOut.String())
	}
	if _, err := os.Stat(p1); !os.IsNotExist(err) {
		t.Fatal("a file was written despite two positional arguments")
	}
}

func TestSetUnparseableFlagIsUsageError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key")
	var out, errOut bytes.Buffer
	code := run([]string{"set", path, "--not-a-real-flag"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (stdout: %s, stderr: %s)", code, out.String(), errOut.String())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("a file was written despite an unparseable flag")
	}
}

func TestSetHelpPrintsUsageToStdoutAndExitsZero(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"set", "-h"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	if errOut.String() != "" {
		t.Fatalf("stderr = %q, want nothing — `cred set -h` is documented as a valid affordance", errOut.String())
	}
	if !strings.Contains(out.String(), "Usage of set") {
		t.Fatalf("stdout = %q, want the set flag set's usage", out.String())
	}
}

// --- expandTilde ---

func TestExpandTildeExpandsBareTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home directory available: %v", err)
	}
	got, err := expandTilde("~")
	if err != nil {
		t.Fatal(err)
	}
	if got != home {
		t.Fatalf("expandTilde(~) = %q, want %q", got, home)
	}
}

func TestExpandTildeExpandsTildeSlash(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home directory available: %v", err)
	}
	got, err := expandTilde("~/sub/key")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "sub", "key")
	if got != want {
		t.Fatalf("expandTilde(~/sub/key) = %q, want %q", got, want)
	}
}

func TestExpandTildeLeavesOtherPathsAlone(t *testing.T) {
	got, err := expandTilde("/abs/path")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/abs/path" {
		t.Fatalf("expandTilde(/abs/path) = %q, want unchanged", got)
	}
}

func TestExpandTildeRejectsOtherUsersHome(t *testing.T) {
	for _, p := range []string{"~alice/x", "~bob"} {
		if _, err := expandTilde(p); err == nil {
			t.Fatalf("expandTilde(%q) = nil error, want a rejection — only ~/ is supported", p)
		}
	}
}

func TestExpandTildeSurfacesUserHomeDirError(t *testing.T) {
	t.Setenv("HOME", "")
	if _, err := expandTilde("~/x"); err == nil {
		t.Fatal("want an error when the home directory can't be determined, not a silent fall-through to a literal ~ path")
	}
}

func TestSetUserHomeDirFailureIsRuntimeErrorNotAJunkPath(t *testing.T) {
	dir := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldwd)
	t.Setenv("HOME", "")

	var out, errOut bytes.Buffer
	code := run([]string{"set", "~/x", "--value-from", "printf 'cal_live_abc'"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (stdout: %s, stderr: %s)", code, out.String(), errOut.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "~")); !os.IsNotExist(err) {
		t.Fatal("a literal ~ path was created despite the UserHomeDir failure")
	}
}
