package store

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeEnv(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func readEnv(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSetEnvKeyReplacesInPlacePreservingEverythingElse(t *testing.T) {
	p := writeEnv(t, "# a comment\nAPP_ENV=local\nAPI_KEY=old\nDB_HOST=localhost\n")
	if err := SetEnvKey(p, "API_KEY", "new", 0o600); err != nil {
		t.Fatal(err)
	}
	want := "# a comment\nAPP_ENV=local\nAPI_KEY=new\nDB_HOST=localhost\n"
	if got := readEnv(t, p); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestSetEnvKeyAppendsWhenAbsent(t *testing.T) {
	p := writeEnv(t, "APP_ENV=local\n")
	if err := SetEnvKey(p, "API_KEY", "new", 0o600); err != nil {
		t.Fatal(err)
	}
	if got, want := readEnv(t, p), "APP_ENV=local\nAPI_KEY=new\n"; got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestSetEnvKeyAppendsNewlineWhenFileLacksTrailingOne(t *testing.T) {
	p := writeEnv(t, "APP_ENV=local")
	if err := SetEnvKey(p, "API_KEY", "new", 0o600); err != nil {
		t.Fatal(err)
	}
	if got, want := readEnv(t, p), "APP_ENV=local\nAPI_KEY=new\n"; got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestSetEnvKeyCreatesTheFileWhenMissing(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".env")
	if err := SetEnvKey(p, "API_KEY", "new", 0o600); err != nil {
		t.Fatal(err)
	}
	if got, want := readEnv(t, p), "API_KEY=new\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
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

		// The double-quote escaping replacer only fires once a value
		// contains a single quote (which forces the double-quoted branch);
		// each case below pairs a single quote with one of the characters
		// the replacer escapes.
		"back\\slash'quote": "API_KEY=\"back\\\\slash'quote\"\n", // backslash
		"cost$5'off":        "API_KEY=\"cost\\$5'off\"\n",        // dollar
		"cmd`sub'x":         "API_KEY=\"cmd\\`sub'x\"\n",         // backtick
		"say\"hi'there":     "API_KEY=\"say\\\"hi'there\"\n",     // double quote, and both quote types together
		"o'clock\\":         "API_KEY=\"o'clock\\\\\"\n",         // ends in a backslash
	}
	for value, want := range cases {
		p := filepath.Join(t.TempDir(), ".env")
		if err := SetEnvKey(p, "API_KEY", value, 0o600); err != nil {
			t.Fatalf("%q: %v", value, err)
		}
		if got := readEnv(t, p); got != want {
			t.Errorf("value %q -> %q, want %q", value, got, want)
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
	p := writeEnv(t, "MY_API_KEY=untouched\nAPI_KEY=old\n")
	if err := SetEnvKey(p, "API_KEY", "new", 0o600); err != nil {
		t.Fatal(err)
	}
	if got, want := readEnv(t, p), "MY_API_KEY=untouched\nAPI_KEY=new\n"; got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestSetEnvKeyPreservesAnExistingWiderModeChoice(t *testing.T) {
	p := writeEnv(t, "APP_ENV=local\n")
	if err := os.Chmod(p, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SetEnvKey(p, "API_KEY", "new", 0o600); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o644 {
		t.Fatalf("mode = %o, want the file's existing 644 preserved", fi.Mode().Perm())
	}
}

func TestGetEnvKeyUnquotes(t *testing.T) {
	p := writeEnv(t, "A=plain\nB='single quoted'\nC=\"double quoted\"\n")
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
	p := writeEnv(t, "A=plain\n")
	if _, err := GetEnvKey(p, "NOPE"); err == nil {
		t.Fatal("want an error for a missing key")
	}
}

func TestRemoveEnvKeyDropsOnlyThatLine(t *testing.T) {
	p := writeEnv(t, "# keep\nA=1\nAPI_KEY=secret\nB=2\n")
	if err := RemoveEnvKey(p, "API_KEY"); err != nil {
		t.Fatal(err)
	}
	if got, want := readEnv(t, p), "# keep\nA=1\nB=2\n"; got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRemoveEnvKeyOnMissingKeyIsANoop(t *testing.T) {
	p := writeEnv(t, "A=1\nB=2\n")
	if err := RemoveEnvKey(p, "NOPE"); err != nil {
		t.Fatal(err)
	}
	if got, want := readEnv(t, p), "A=1\nB=2\n"; got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRemoveEnvKeyOnMissingFileIsAnError(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".env")
	if err := RemoveEnvKey(p, "API_KEY"); err == nil {
		t.Fatal("want an error when the file does not exist")
	}
	if _, statErr := os.Stat(p); !os.IsNotExist(statErr) {
		t.Fatal("RemoveEnvKey must not create a file when the source path does not exist")
	}
}

func TestRemoveEnvKeyOnExisting0644FileLeavesIt0644(t *testing.T) {
	p := writeEnv(t, "A=1\nAPI_KEY=secret\nB=2\n")
	if err := os.Chmod(p, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RemoveEnvKey(p, "API_KEY"); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o644 {
		t.Fatalf("mode = %o, want the file's existing 644 preserved", fi.Mode().Perm())
	}
}

// Set->Get round trip: whatever SetEnvKey was given, GetEnvKey must return
// back exactly, across the value shapes cred is likely to see in practice.
func TestSetThenGetRoundTrips(t *testing.T) {
	values := []string{
		"plaintoken",
		"has space",
		"has#hash",
		"has=equals",
		"has$dollar",
		"has`backtick",
		`has\backslash`,
		`trailing\`,
		"has'single",
		`has"double`,
		`both'and"quotes`,
		" leading",
		"trailing ",
		"",
	}
	values = append(values, shellMetacharValues()...)
	for _, value := range values {
		p := filepath.Join(t.TempDir(), ".env")
		if err := SetEnvKey(p, "API_KEY", value, 0o600); err != nil {
			t.Fatalf("Set(%q): %v", value, err)
		}
		got, err := GetEnvKey(p, "API_KEY")
		if err != nil {
			t.Fatalf("Get after Set(%q): %v", value, err)
		}
		if got != value {
			t.Errorf("round trip for %q: got %q", value, got)
		}
	}
}

func TestSetEnvKeyRefusesADuplicateKey(t *testing.T) {
	p := writeEnv(t, "API_KEY=first\nAPI_KEY=second\n")
	err := SetEnvKey(p, "API_KEY", "new", 0o600)
	if err == nil {
		t.Fatal("want an error for a duplicate key")
	}
	if !strings.Contains(err.Error(), "API_KEY") {
		t.Errorf("error should name the key, got: %v", err)
	}
	if !strings.Contains(err.Error(), p) {
		t.Errorf("error should name the path, got: %v", err)
	}
	if !strings.Contains(err.Error(), "2") {
		t.Errorf("error should name the occurrence count, got: %v", err)
	}
	if strings.Contains(err.Error(), "first") || strings.Contains(err.Error(), "second") {
		t.Errorf("error must not leak a value, got: %v", err)
	}
	if got, want := readEnv(t, p), "API_KEY=first\nAPI_KEY=second\n"; got != want {
		t.Fatalf("SetEnvKey wrote something despite the duplicate:\n%s", got)
	}
}

func TestSetEnvKeyRefusesADuplicateKeyWithExportOrLeadingWhitespace(t *testing.T) {
	p := writeEnv(t, "API_KEY=one\nexport API_KEY=two\n")
	if err := SetEnvKey(p, "API_KEY", "new", 0o600); err == nil {
		t.Fatal("want an error for a duplicate expressed as a plain assignment and an export")
	}
	if got, want := readEnv(t, p), "API_KEY=one\nexport API_KEY=two\n"; got != want {
		t.Fatalf("SetEnvKey wrote something despite the duplicate:\n%s", got)
	}

	p2 := writeEnv(t, "API_KEY=one\n  API_KEY=two\n")
	if err := SetEnvKey(p2, "API_KEY", "new", 0o600); err == nil {
		t.Fatal("want an error for a duplicate with leading whitespace")
	}
	if got, want := readEnv(t, p2), "API_KEY=one\n  API_KEY=two\n"; got != want {
		t.Fatalf("SetEnvKey wrote something despite the duplicate:\n%s", got)
	}
}

func TestGetEnvKeyRefusesADuplicateKey(t *testing.T) {
	p := writeEnv(t, "API_KEY=first\nAPI_KEY=second\n")
	_, err := GetEnvKey(p, "API_KEY")
	if err == nil {
		t.Fatal("want an error for a duplicate key")
	}
	if !strings.Contains(err.Error(), "API_KEY") {
		t.Errorf("error should name the key, got: %v", err)
	}
	if !strings.Contains(err.Error(), p) {
		t.Errorf("error should name the path, got: %v", err)
	}
	if strings.Contains(err.Error(), "first") || strings.Contains(err.Error(), "second") {
		t.Errorf("error must not leak a value, got: %v", err)
	}
}

func TestRemoveEnvKeyDeletesAllDuplicateOccurrences(t *testing.T) {
	p := writeEnv(t, "A=1\nAPI_KEY=first\nB=2\nAPI_KEY=second\nC=3\n")
	if err := RemoveEnvKey(p, "API_KEY"); err != nil {
		t.Fatal(err)
	}
	if got, want := readEnv(t, p), "A=1\nB=2\nC=3\n"; got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestSetEnvKeyPreservesCRLFLineEndings(t *testing.T) {
	p := writeEnv(t, "APP_ENV=local\r\nAPI_KEY=old\r\nDB_HOST=localhost\r\n")
	if err := SetEnvKey(p, "API_KEY", "new", 0o600); err != nil {
		t.Fatal(err)
	}
	want := "APP_ENV=local\r\nAPI_KEY=new\r\nDB_HOST=localhost\r\n"
	if got := readEnv(t, p); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRemoveEnvKeyPreservesCRLFLineEndings(t *testing.T) {
	p := writeEnv(t, "A=1\r\nAPI_KEY=secret\r\nB=2\r\n")
	if err := RemoveEnvKey(p, "API_KEY"); err != nil {
		t.Fatal(err)
	}
	want := "A=1\r\nB=2\r\n"
	if got := readEnv(t, p); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSetEnvKeyOnANewFileUsesLFRegardlessOfPlatform(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".env")
	if err := SetEnvKey(p, "API_KEY", "new", 0o600); err != nil {
		t.Fatal(err)
	}
	if got, want := readEnv(t, p), "API_KEY=new\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestGetEnvKeyHandlesALineLongerThanTheScannerDefault(t *testing.T) {
	long := strings.Repeat("x", 100*1024)
	p := writeEnv(t, "COMMENT="+long+"\nAPI_KEY=value\n")
	got, err := GetEnvKey(p, "API_KEY")
	if err != nil {
		t.Fatal(err)
	}
	if got != "value" {
		t.Fatalf("got %q, want %q", got, "value")
	}
}

func TestRemoveEnvKeyHandlesALineLongerThanTheScannerDefault(t *testing.T) {
	long := strings.Repeat("x", 100*1024)
	p := writeEnv(t, "COMMENT="+long+"\nAPI_KEY=value\n")
	if err := RemoveEnvKey(p, "API_KEY"); err != nil {
		t.Fatal(err)
	}
	want := "COMMENT=" + long + "\n"
	if got := readEnv(t, p); got != want {
		t.Fatal("long line was not preserved")
	}
}

func TestEnvKeyMatchesWhitespaceAroundTheEquals(t *testing.T) {
	const body = "A=1\nAPI_KEY = old\nB=2\n"

	p := writeEnv(t, body)
	if has, err := HasEnvKey(p, "API_KEY"); err != nil || !has {
		t.Fatalf("HasEnvKey = %v, %v; want true, nil", has, err)
	}
	if got, err := GetEnvKey(p, "API_KEY"); err != nil || got != "old" {
		t.Fatalf("GetEnvKey = %q, %v; want \"old\", nil", got, err)
	}

	if err := SetEnvKey(p, "API_KEY", "new", 0o600); err != nil {
		t.Fatal(err)
	}
	if got, want := readEnv(t, p), "A=1\nAPI_KEY=new\nB=2\n"; got != want {
		t.Fatalf("set replaced into:\n%q\nwant:\n%q", got, want)
	}

	p = writeEnv(t, body)
	if err := RemoveEnvKey(p, "API_KEY"); err != nil {
		t.Fatal(err)
	}
	if got, want := readEnv(t, p), "A=1\nB=2\n"; got != want {
		t.Fatalf("rm left:\n%q\nwant:\n%q", got, want)
	}
}

func TestEnvKeyWhitespaceMatchStaysAnchoredOnTheWholeKey(t *testing.T) {
	p := writeEnv(t, "API_KEY_2 = untouched\n")
	if has, err := HasEnvKey(p, "API_KEY"); err != nil || has {
		t.Fatalf("HasEnvKey = %v, %v; want false, nil", has, err)
	}
}

// shellMetacharValues pairs every shellSpecial character with a payload that
// creates the file "pwned" if sh ever interprets it, plus the word-start and
// mixed-quote shapes.
func shellMetacharValues() []string {
	var vs []string
	for _, c := range shellSpecial {
		vs = append(vs, "ab"+string(c)+"touch pwned", string(c)+"touch pwned")
	}
	return append(vs,
		"ab&cd;rm -rf x",
		"$(touch pwned)",
		"`touch pwned`",
		">pwned",
		"a|touch pwned",
		"=leading-equals",
		"~/home-relative",
		"x;touch pwned'quoted",
		"it's;$(touch pwned)&`touch pwned`|x",
		"sk_live_abc-123.XYZ+/:@%^,",
	)
}

func TestSetEnvKeyQuotesShellMetacharacters(t *testing.T) {
	for _, c := range ";&|<>()*?[]{}~!" {
		v := "ab" + string(c) + "cd"
		if got := quoteEnv(v); got != "'"+v+"'" {
			t.Errorf("quoteEnv(%q) = %q, want it single-quoted", v, got)
		}
	}
}

// A file cred writes is safe to `.`-source: every value comes back exactly,
// and no payload in it runs.
func TestSetEnvKeyOutputIsSafeToSourceInSh(t *testing.T) {
	var shells []string
	for _, sh := range []string{"sh", "dash", "bash"} {
		if _, err := exec.LookPath(sh); err == nil {
			shells = append(shells, sh)
		}
	}
	if len(shells) == 0 {
		t.Skip("no POSIX shell on PATH")
	}
	for _, value := range shellMetacharValues() {
		dir := t.TempDir()
		if err := SetEnvKey(filepath.Join(dir, ".env"), "K", value, 0o600); err != nil {
			t.Fatalf("Set(%q): %v", value, err)
		}
		for _, sh := range shells {
			cmd := exec.Command(sh, "-c", `. ./.env; printf %s "$K"`)
			cmd.Dir = dir
			out, err := cmd.Output()
			if err != nil {
				t.Errorf("%s sourcing %q: %v", sh, value, err)
			}
			if string(out) != value {
				t.Errorf("%s sourced %q as %q", sh, value, out)
			}
			if _, err := os.Stat(filepath.Join(dir, "pwned")); err == nil {
				t.Fatalf("%s sourcing %q ran its payload", sh, value)
			}
		}
	}
}

// A hand-edited trailing comment is not part of the value: GetEnvKey reads
// each line the way `. ./.env` does.
func TestGetEnvKeyDropsATrailingComment(t *testing.T) {
	cases := map[string]string{
		"K=sk_abc # prod key":          "sk_abc",
		"K=sk_abc\t#prod":              "sk_abc",
		"K=sk_abc #":                   "sk_abc",
		"K=a#b":                        "a#b",
		"K=#abc":                       "#abc",
		"K='q #in' # c":                "q #in",
		"K='q #in'":                    "q #in",
		`K="d \"#\" q" # c`:            `d "#" q`,
		`K="it's" #c`:                  "it's",
		"export K=sk_abc  # two-space": "sk_abc",
	}
	_, shErr := exec.LookPath("sh")
	for line, want := range cases {
		p := writeEnv(t, line+"\n")
		got, err := GetEnvKey(p, "K")
		if err != nil {
			t.Fatalf("%q: %v", line, err)
		}
		if got != want {
			t.Errorf("%q -> %q, want %q", line, got, want)
		}
		if shErr != nil {
			continue
		}
		cmd := exec.Command("sh", "-c", `. "$1"; printf %s "$K"`, "sh", p)
		if out, err := cmd.Output(); err != nil || string(out) != want {
			t.Errorf("sh reads %q as %q (err %v), want %q", line, out, err, want)
		}
	}
}
