package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lockyc/cred/internal/receipt"
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
	if !strings.Contains(out.String(), row("key", "API_KEY")) {
		t.Fatalf("receipt missing exact key row %q:\n%s", row("key", "API_KEY"), out.String())
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

// Unlike TestSetUserHomeDirFailureIsRuntimeErrorNotAJunkPath just above —
// where UserHomeDir itself fails, a genuine runtime error — a `~user` form
// is a malformed argument expandTilde already refuses on its own terms. It
// must be a usage error (2), not lumped in with the runtime-error path (1)
// that the two share today.
func TestSetOtherUsersHomeIsUsageError(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"set", "~alice/x", "--value-from", "printf 'cal_live_abc'"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (stdout: %s, stderr: %s)", code, out.String(), errOut.String())
	}
}

// --- show and rm ---

func TestShowReportsWithoutTheValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key")
	var discard bytes.Buffer
	run([]string{"set", path, "--value-from", "printf 'cal_live_SECRET'"}, &discard, &discard)

	// Capture the file's actual mtime so the "modified" row can be asserted
	// exactly, the same way row("mode", "600") is: a loose Contains("modified")
	// would also pass if the word only showed up inside a t.TempDir() path.
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	wantModified := fi.ModTime().Format("2006-01-02 15:04:05")

	var out, errOut bytes.Buffer
	if code := run([]string{"show", path}, &out, &errOut); code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut.String())
	}
	if strings.Contains(out.String(), "SECRET") {
		t.Fatalf("show leaked the value on stdout:\n%s", out.String())
	}
	if strings.Contains(errOut.String(), "SECRET") {
		t.Fatalf("show leaked the value on stderr:\n%s", errOut.String())
	}
	if errOut.String() != "" {
		t.Fatalf("show wrote to stderr on success: %q", errOut.String())
	}
	if !strings.Contains(out.String(), "cred: present") {
		t.Fatalf("show missing the header:\n%s", out.String())
	}
	if !strings.Contains(out.String(), row("mode", "600")) {
		t.Fatalf("show missing exact mode row %q:\n%s", row("mode", "600"), out.String())
	}
	if !strings.Contains(out.String(), row("modified", wantModified)) {
		t.Fatalf("show missing exact modified row %q:\n%s", row("modified", wantModified), out.String())
	}
}

func TestShowMissingFileExitsOne(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"show", filepath.Join(t.TempDir(), "nope")}, &out, &errOut)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	// Asserted against stdout only, not out+errOut: a MISSING report that
	// regressed onto stderr must fail this test, not slip past it.
	if !strings.Contains(out.String(), "MISSING") {
		t.Fatalf("want a MISSING report on stdout, got stdout:\n%s\nstderr:\n%s", out.String(), errOut.String())
	}
}

// A stat failure that isn't "the file doesn't exist" (EACCES on a parent
// directory here) must not be reported as MISSING: a user told a live,
// merely-unreadable credential is MISSING may go run `cred set` and
// overwrite it. Removing search permission on the parent directory is a
// reliable way to force a non-ENOENT stat error without touching the file
// itself; root bypasses the permission check, so this only means something
// as a non-root user.
func TestShowNonNotExistStatErrorIsRuntimeErrorNotMissing(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: directory permission bits don't block stat")
	}
	dir := t.TempDir()
	locked := filepath.Join(dir, "locked")
	if err := os.Mkdir(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(locked, "key")
	if err := os.WriteFile(path, []byte("cal_live_x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) }) // let t.TempDir() clean up

	var out, errOut bytes.Buffer
	code := run([]string{"show", path}, &out, &errOut)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (stdout: %s, stderr: %s)", code, out.String(), errOut.String())
	}
	// The discriminating assertion: the old code reported every stat error,
	// including this one, as MISSING. Exit 1 alone doesn't catch that — both
	// the old and new code return 1 here.
	if strings.Contains(out.String(), "MISSING") {
		t.Fatalf("a permission-denied stat was reported as MISSING, masking the real error:\n%s", out.String())
	}
	// "cred:" alone is satisfied by every cred error, including the old
	// MISSING report — assert the actual underlying stat error text so this
	// only passes when the real error was surfaced.
	if !strings.Contains(errOut.String(), "permission denied") {
		t.Fatalf("stderr = %q, want the underlying \"permission denied\" stat error surfaced", errOut.String())
	}
}

func TestShowFingerprintMatchesSetFingerprint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key")
	var setOut, discard bytes.Buffer
	run([]string{"set", path, "--value-from", "printf 'cal_live_abc'"}, &setOut, &discard)
	var showOut bytes.Buffer
	run([]string{"show", path}, &showOut, &discard)

	fp := receipt.Fingerprint("cal_live_abc")
	want := row("fingerprint", fp)
	if !strings.Contains(setOut.String(), want) || !strings.Contains(showOut.String(), want) {
		t.Fatalf("fingerprints disagree (want exact row %q):\nset:\n%s\nshow:\n%s", want, setOut.String(), showOut.String())
	}
}

func TestRmDeletesTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key")
	var discard bytes.Buffer
	run([]string{"set", path, "--value-from", "printf 'x'"}, &discard, &discard)
	if code := run([]string{"rm", path}, &discard, &discard); code != 0 {
		t.Fatal("rm failed")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("file still exists after rm")
	}
}

// os.Stat follows a symlink, so before the fix `rm` on a symlink pointing at
// a real credential passed the IsRegular guard (stat sees the *target*'s
// mode) and os.Remove unlinked the symlink — printing "removed", exit 0,
// while the credential itself survived untouched at its target path. The
// discriminating assertions are the exit code, the absence of "removed" on
// stdout, and — the part a mere exit-code check would miss — that both the
// link and the target are still there afterward with the target's content
// unchanged.
func TestRmRefusesASymlinkAndLeavesTargetInPlace(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("cal_live_secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := run([]string{"rm", link}, &out, &errOut)
	if code == 0 {
		t.Fatalf("exit = 0, want a refusal (stdout: %s, stderr: %s)", out.String(), errOut.String())
	}
	if strings.Contains(out.String(), "removed") {
		t.Fatalf("stdout claims a removal that must not have happened: %q", out.String())
	}
	if _, err := os.Lstat(link); err != nil {
		t.Fatalf("the symlink itself was removed: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("the credential the symlink pointed at is gone: %v", err)
	}
	if string(got) != "cal_live_secret\n" {
		t.Fatalf("target contents changed: %q", got)
	}
}

func TestRmRemovesOnlyTheNamedEnvKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("A=1\nAPI_KEY=secret\nB=2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var discard bytes.Buffer
	if code := run([]string{"rm", path, "--name", "API_KEY"}, &discard, &discard); code != 0 {
		t.Fatal("rm failed")
	}
	b, _ := os.ReadFile(path)
	if string(b) != "A=1\nB=2\n" {
		t.Fatalf("env file =\n%s", string(b))
	}
}

// rm on an absent path must report MISSING the same way show does — same
// stream (stdout), same word — rather than a raw "no such file" on stderr.
// Exit code alone doesn't discriminate this (both the old and new behaviour
// return 1); the stdout/MISSING assertions are what would have caught the
// old divergence.
func TestRmMissingFileReportsMissingOnStdout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope")
	var out, errOut bytes.Buffer
	code := run([]string{"rm", path}, &out, &errOut)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (stdout: %s, stderr: %s)", code, out.String(), errOut.String())
	}
	if strings.Contains(out.String(), "removed") {
		t.Fatalf("stdout claims a removal that didn't happen: %q", out.String())
	}
	if !strings.Contains(out.String(), "MISSING") {
		t.Fatalf("want a MISSING report on stdout, got stdout:\n%s\nstderr:\n%s", out.String(), errOut.String())
	}
	if errOut.String() != "" {
		t.Fatalf("stderr = %q, want nothing — MISSING is curated, not a raw error", errOut.String())
	}
}

// The same MISSING/stdout report applies to `rm --name`, whose absent-file
// case previously reached HasEnvKey's raw os.ReadFile error on stderr
// instead of the curated report show and whole-file rm both use.
func TestRmNameOnMissingFileReportsMissingOnStdout(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	var out, errOut bytes.Buffer
	code := run([]string{"rm", path, "--name", "API_KEY"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (stdout: %s, stderr: %s)", code, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), "MISSING") {
		t.Fatalf("want a MISSING report on stdout, got stdout:\n%s\nstderr:\n%s", out.String(), errOut.String())
	}
	if errOut.String() != "" {
		t.Fatalf("stderr = %q, want nothing — MISSING is curated, not a raw error", errOut.String())
	}
}

func TestRmRefusesADirectory(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "somedir")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code := run([]string{"rm", target}, &out, &errOut)
	if code == 0 {
		t.Fatalf("exit = 0, want a refusal (stdout: %s, stderr: %s)", out.String(), errOut.String())
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("directory was removed despite not being a regular file: %v", err)
	}
	if strings.Contains(out.String(), "removed") {
		t.Fatalf("stdout claims a removal that must not have happened: %q", out.String())
	}
}

// Before the fix, `set` on a directory reached os.Rename inside
// writeFileAtomic and surfaced a raw "rename ...: is a directory" error —
// still exit 1, but not in cred's own voice. It's also checked up front, so
// no TTY prompt happens first (no --value-from is passed here on purpose:
// if the check ran too late, this would hang or fail on "needs a terminal"
// instead of the curated refusal).
func TestSetRefusesADirectory(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "somedir")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code := run([]string{"set", target}, &out, &errOut)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (stdout: %s, stderr: %s)", code, out.String(), errOut.String())
	}
	if !strings.Contains(errOut.String(), "not a regular file") {
		t.Fatalf("stderr = %q, want cred's own curated refusal, not a raw OS error", errOut.String())
	}
	fi, err := os.Stat(target)
	if err != nil || !fi.IsDir() {
		t.Fatalf("target is no longer the directory it was: %v", err)
	}
}

// Before the fix, `show` on a directory reached store.ReadFile's raw
// "read ...: is a directory" error instead of a curated refusal matching
// rm's.
func TestShowRefusesADirectory(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "somedir")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code := run([]string{"show", target}, &out, &errOut)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (stdout: %s, stderr: %s)", code, out.String(), errOut.String())
	}
	if !strings.Contains(errOut.String(), "not a regular file") {
		t.Fatalf("stderr = %q, want cred's own curated refusal, not a raw OS error", errOut.String())
	}
	if strings.Contains(out.String(), "MISSING") {
		t.Fatalf("a directory was reported as MISSING: %q", out.String())
	}
}

// set and show refuse a symlink the same way rm does (TestRmRefusesA
// SymlinkAndLeavesTargetInPlace) — cred never follows a symlink to write or
// read through it, so the three commands agree on what "not a regular file"
// covers.
func TestSetAndShowRefuseASymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("cal_live_secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	var setOut, setErr bytes.Buffer
	if code := run([]string{"set", link}, &setOut, &setErr); code != 1 {
		t.Fatalf("set: exit = %d, want 1 (stdout: %s, stderr: %s)", code, setOut.String(), setErr.String())
	}
	if !strings.Contains(setErr.String(), "not a regular file") {
		t.Fatalf("set stderr = %q, want the curated refusal", setErr.String())
	}

	var showOut, showErr bytes.Buffer
	if code := run([]string{"show", link}, &showOut, &showErr); code != 1 {
		t.Fatalf("show: exit = %d, want 1 (stdout: %s, stderr: %s)", code, showOut.String(), showErr.String())
	}
	if !strings.Contains(showErr.String(), "not a regular file") {
		t.Fatalf("show stderr = %q, want the curated refusal", showErr.String())
	}

	got, err := os.ReadFile(target)
	if err != nil || string(got) != "cal_live_secret\n" {
		t.Fatalf("target changed: %q, %v", got, err)
	}
}

// store.RemoveEnvKey is documented as a no-op when the key is absent, so
// before the fix `cred rm --name` on a missing key printed "removed key
// NOPE" and exited 0 having removed nothing. The discriminating assertions
// are the exact message and exit code, not just "not zero" — a test that
// only checked the exit code wouldn't distinguish an honest refusal from any
// other unrelated failure.
//
// The fixture deliberately has no trailing newline: RemoveEnvKey's rewrite
// path (via writeFileAtomic) normalises line endings, so a fixture already
// in normalised form ("A=1\n") would pass even if runRm called RemoveEnvKey
// unconditionally and only checked presence afterward to decide what to
// print — the spurious rewrite would produce byte-identical output. The
// mtime check below is the same belt-and-braces: even if content happened
// to come out identical, an unwanted rewrite still replaces the inode.
func TestRmNameOnAbsentKeyDoesNotClaimSuccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("A=1"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code := run([]string{"rm", path, "--name", "NOPE"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (stdout: %s, stderr: %s)", code, out.String(), errOut.String())
	}
	if strings.Contains(out.String(), "removed") {
		t.Fatalf("stdout falsely claims a removal: %q", out.String())
	}
	if !strings.Contains(errOut.String(), "NOPE") || !strings.Contains(errOut.String(), "not set") {
		t.Fatalf("stderr = %q, want an honest report that NOPE was never set", errOut.String())
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "A=1" {
		t.Fatalf("env file changed despite the key being absent =\n%s", string(b))
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatalf("file was rewritten despite the key being absent: mtime %v -> %v", before.ModTime(), after.ModTime())
	}
}

func TestRmNameOnDuplicateKeyRemovesAllOccurrences(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("API_KEY=one\nAPI_KEY=two\nB=2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code := run([]string{"rm", path, "--name", "API_KEY"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stdout: %s, stderr: %s)", code, out.String(), errOut.String())
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "B=2\n" {
		t.Fatalf("env file =\n%s, want every API_KEY occurrence gone", string(b))
	}
	if !strings.Contains(out.String(), "removed") {
		t.Fatalf("stdout = %q, want an honest removal report", out.String())
	}
}

func TestShowDuplicateEnvKeyIsExitOne(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("API_KEY=one\nAPI_KEY=two\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code := run([]string{"show", path, "--name", "API_KEY"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (stdout: %s, stderr: %s)", code, out.String(), errOut.String())
	}
	// Exit 1 alone doesn't pin this down — it's also what the MISSING branch
	// returns, so a regression that mistook "duplicate" for "absent" would
	// still pass on the exit code. Pin the actual refusal text and that
	// nothing else (in particular no value) reached stdout.
	if !strings.Contains(errOut.String(), `"API_KEY"`) || !strings.Contains(errOut.String(), "appears 2 times") {
		t.Fatalf("stderr = %q, want the duplicate-key refusal naming the key and occurrence count", errOut.String())
	}
	if out.String() != "" {
		t.Fatalf("stdout = %q, want nothing on a refused duplicate-key read", out.String())
	}
}

func TestShowFlagsAndPathInEitherOrder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("API_KEY=sk_live_x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out1, errOut1 bytes.Buffer
	if code := run([]string{"show", "--name", "API_KEY", path}, &out1, &errOut1); code != 0 {
		t.Fatalf("flags-before-path: exit = %d, stderr = %s", code, errOut1.String())
	}
	var out2, errOut2 bytes.Buffer
	if code := run([]string{"show", path, "--name", "API_KEY"}, &out2, &errOut2); code != 0 {
		t.Fatalf("path-before-flags: exit = %d, stderr = %s", code, errOut2.String())
	}
	want := row("key", "API_KEY")
	if !strings.Contains(out1.String(), want) || !strings.Contains(out2.String(), want) {
		t.Fatalf("receipt missing exact key row %q:\n%s\n%s", want, out1.String(), out2.String())
	}
}

func TestShowHelpPrintsUsageToStdoutAndExitsZero(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"show", "-h"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	if errOut.String() != "" {
		t.Fatalf("stderr = %q, want nothing", errOut.String())
	}
	if !strings.Contains(out.String(), "Usage of show") {
		t.Fatalf("stdout = %q, want the show flag set's usage", out.String())
	}
}

func TestRmHelpPrintsUsageToStdoutAndExitsZero(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"rm", "-h"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	if errOut.String() != "" {
		t.Fatalf("stderr = %q, want nothing", errOut.String())
	}
	if !strings.Contains(out.String(), "Usage of rm") {
		t.Fatalf("stdout = %q, want the rm flag set's usage", out.String())
	}
}
