package store

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSetEnvKeyReplacesInPlacePreservingEverythingElse(t *testing.T) {
	p := write(t, "# a comment\nAPP_ENV=local\nAPI_KEY=old\nDB_HOST=localhost\n")
	if err := SetEnvKey(p, "API_KEY", "new", 0o600); err != nil {
		t.Fatal(err)
	}
	want := "# a comment\nAPP_ENV=local\nAPI_KEY=new\nDB_HOST=localhost\n"
	if got := read(t, p); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestSetEnvKeyAppendsWhenAbsent(t *testing.T) {
	p := write(t, "APP_ENV=local\n")
	if err := SetEnvKey(p, "API_KEY", "new", 0o600); err != nil {
		t.Fatal(err)
	}
	if got, want := read(t, p), "APP_ENV=local\nAPI_KEY=new\n"; got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestSetEnvKeyAppendsNewlineWhenFileLacksTrailingOne(t *testing.T) {
	p := write(t, "APP_ENV=local")
	if err := SetEnvKey(p, "API_KEY", "new", 0o600); err != nil {
		t.Fatal(err)
	}
	if got, want := read(t, p), "APP_ENV=local\nAPI_KEY=new\n"; got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestSetEnvKeyCreatesTheFileWhenMissing(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".env")
	if err := SetEnvKey(p, "API_KEY", "new", 0o600); err != nil {
		t.Fatal(err)
	}
	if got, want := read(t, p), "API_KEY=new\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	fi, _ := os.Stat(p)
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 600", fi.Mode().Perm())
	}
}

func TestSetEnvKeyQuotesValuesThatNeedIt(t *testing.T) {
	cases := map[string]string{
		"plainvalue": "API_KEY=plainvalue\n",
		"has space":  "API_KEY='has space'\n",
		"has#hash":   "API_KEY='has#hash'\n",
		"has'quote":  "API_KEY=\"has'quote\"\n",
	}
	for value, want := range cases {
		p := filepath.Join(t.TempDir(), ".env")
		if err := SetEnvKey(p, "API_KEY", value, 0o600); err != nil {
			t.Fatalf("%q: %v", value, err)
		}
		if got := read(t, p); got != want {
			t.Errorf("value %q → %q, want %q", value, got, want)
		}
	}
}

func TestSetEnvKeyRejectsAValueContainingANewline(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".env")
	err := SetEnvKey(p, "API_KEY", "two\nlines", 0o600)
	if err == nil {
		t.Fatal("want an error: a newline cannot be represented portably in a .env")
	}
	if _, statErr := os.Stat(p); !os.IsNotExist(statErr) {
		t.Fatal("the file was created despite the rejected value")
	}
}

func TestSetEnvKeyDoesNotMatchAKeyThatIsASuffix(t *testing.T) {
	p := write(t, "MY_API_KEY=untouched\nAPI_KEY=old\n")
	if err := SetEnvKey(p, "API_KEY", "new", 0o600); err != nil {
		t.Fatal(err)
	}
	if got, want := read(t, p), "MY_API_KEY=untouched\nAPI_KEY=new\n"; got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestSetEnvKeyPreservesAnExistingWiderModeChoice(t *testing.T) {
	p := write(t, "APP_ENV=local\n")
	if err := os.Chmod(p, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SetEnvKey(p, "API_KEY", "new", 0o600); err != nil {
		t.Fatal(err)
	}
	fi, _ := os.Stat(p)
	if fi.Mode().Perm() != 0o644 {
		t.Fatalf("mode = %o, want the file's existing 644 preserved", fi.Mode().Perm())
	}
}

func TestGetEnvKeyUnquotes(t *testing.T) {
	p := write(t, "A=plain\nB='single quoted'\nC=\"double quoted\"\n")
	for key, want := range map[string]string{"A": "plain", "B": "single quoted", "C": "double quoted"} {
		got, err := GetEnvKey(p, key)
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		if got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestGetEnvKeyMissingIsAnError(t *testing.T) {
	p := write(t, "A=plain\n")
	if _, err := GetEnvKey(p, "NOPE"); err == nil {
		t.Fatal("want an error for a missing key")
	}
}

func TestRemoveEnvKeyDropsOnlyThatLine(t *testing.T) {
	p := write(t, "# keep\nA=1\nAPI_KEY=secret\nB=2\n")
	if err := RemoveEnvKey(p, "API_KEY"); err != nil {
		t.Fatal(err)
	}
	if got, want := read(t, p), "# keep\nA=1\nB=2\n"; got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}
