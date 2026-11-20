package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileRefusesAFileHoldingEnvAssignments(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".env")
	const body = "# app\nexport API_KEY=old\nOTHER=keep\n"
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(p, "new", 0o600); err == nil {
		t.Fatal("WriteFile replaced a .env-shaped file")
	}
	if got, _ := os.ReadFile(p); string(got) != body {
		t.Fatalf("file changed: %q", got)
	}
	tok := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tok, []byte("sk-old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(tok, "sk-new", 0o600); err != nil {
		t.Fatalf("a standalone credential still rotates: %v", err)
	}
}

func TestWriteFileCreatesWithRequestedMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "key")
	if err := WriteFile(path, "tok_abc", 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := fi.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %o, want 600", got)
	}
}

func TestWriteFileRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key")
	want := "tok_abc"
	if err := WriteFile(path, want, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWriteFileOverwritesAndNeverWidensMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key")
	if err := WriteFile(path, "first", 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(path, "second", 0o600); err != nil {
		t.Fatal(err)
	}
	got, _ := ReadFile(path)
	if got != "second" {
		t.Fatalf("got %q, want %q", got, "second")
	}
	fi, _ := os.Stat(path)
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 600", fi.Mode().Perm())
	}
}

func TestReadFileStripsTheTrailingNewline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(path, []byte("tok_abc\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != "tok_abc" {
		t.Fatalf("got %q, want %q", got, "tok_abc")
	}
}
