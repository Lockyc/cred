package store

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/joho/godotenv"
)

// FuzzQuoteEnv: quoteEnv refuses, or produces exactly one of the three forms
// it documents, and that form reads back as the value in unquoteEnv and in
// godotenv.
func FuzzQuoteEnv(f *testing.F) {
	for _, v := range readerCorpus() {
		f.Add(v)
	}
	f.Add("it's$5")
	f.Add(`o'clock\`)
	f.Add(`it's "q"`)
	f.Fuzz(func(t *testing.T, v string) {
		q, err := quoteEnv(v)
		if err != nil {
			return
		}
		switch {
		case v == "":
			if q != `''` {
				t.Fatalf("quoteEnv(\"\") = %q", q)
			}
		case !strings.ContainsAny(v, shellSpecial) && strings.TrimSpace(v) == v:
			if q != v {
				t.Fatalf("quoteEnv(%q) = %q, want it bare", v, q)
			}
		case !strings.ContainsAny(v, `'\`):
			if q != "'"+v+"'" {
				t.Fatalf("quoteEnv(%q) = %q, want it single-quoted", v, q)
			}
		default:
			if strings.ContainsAny(v, "$`") || strings.HasSuffix(v, `\`) || strings.HasSuffix(v, `"`) {
				t.Fatalf("quoteEnv(%q) = %q, want a refusal", v, q)
			}
			if want := `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(v) + `"`; q != want {
				t.Fatalf("quoteEnv(%q) = %q, want %q", v, q, want)
			}
		}
		if got, ok := unquoteEnv(q); !ok || got != v {
			t.Fatalf("unquoteEnv(quoteEnv(%q)) = %q (encoded %q)", v, got, q)
		}
		if strings.ContainsAny(v, "\r\n") || !utf8.ValidString(v) {
			return
		}
		env, err := godotenv.Unmarshal("K=" + q + "\n")
		if err != nil || env["K"] != v {
			t.Fatalf("godotenv reads quoteEnv(%q) = %q as %q (err %v)", v, q, env["K"], err)
		}
	})
}

// envLines splits b the way SetEnvKey reads it, less a trailing CR on each
// line: a lone CR before the end of a line or the file is a line ending the
// rewrite normalises (detectEOL).
func envLines(t *testing.T, b []byte) []string {
	t.Helper()
	var lines []string
	sc := newScanner(b)
	for sc.Scan() {
		lines = append(lines, strings.TrimSuffix(sc.Text(), "\r"))
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return lines
}

// FuzzSetEnvKey: SetEnvKey either refuses and leaves the file's bytes
// unchanged, or K reads back as the value, every other line is kept
// byte-identical and in order, and a replaced line keeps its indentation and
// `export `.
func FuzzSetEnvKey(f *testing.F) {
	f.Add("A=1\nK=old\nB=2\n", "new")
	f.Add("K = old\n", "new")
	f.Add("  export K=old # c\n", "new value")
	f.Add("export\tK\t=\told\r\nB=2\r\n", "a\\b")
	f.Add("K=1\nK=2\n", "x")
	f.Add("A=1", "x")
	f.Add("# K=x\nMY_K=y\nK_2=z\n", "it's")
	f.Add("A=1\r\nB=2\nC=3\r\n", "")
	f.Add("A=1\n", "tok\xffen")
	f.Add("A=1\n", "tok\f")
	f.Fuzz(func(t *testing.T, body, value string) {
		p := filepath.Join(t.TempDir(), ".env")
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := SetEnvKey(p, "K", value, 0o600); err != nil {
			if got, _ := os.ReadFile(p); !bytes.Equal(got, []byte(body)) {
				t.Fatalf("Set refused (%v) but rewrote %q to %q", err, body, got)
			}
			return
		}
		got, err := GetEnvKey(p, "K")
		if err != nil || got != value {
			t.Fatalf("after Set(%q) on %q, Get = %q, %v", value, body, got, err)
		}
		if has, err := HasEnvKey(p, "K"); err != nil || !has {
			t.Fatalf("after Set, HasEnvKey = %v, %v", has, err)
		}

		after, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		re := keyLine("K")
		var before, rest []string
		var prefix string
		for _, l := range envLines(t, []byte(body)) {
			if m := re.FindStringSubmatch(l); m != nil {
				prefix = m[1]
				continue
			}
			before = append(before, l)
		}
		for _, l := range envLines(t, after) {
			if re.MatchString(l) {
				if !strings.HasPrefix(l, prefix+"K=") {
					t.Fatalf("replaced line %q lost its prefix %q", l, prefix)
				}
				if env, err := godotenv.Unmarshal(l); err != nil || env["K"] != value {
					t.Fatalf("godotenv reads %q as %q (err %v), want %q", l, env["K"], err, value)
				}
				continue
			}
			rest = append(rest, l)
		}
		if strings.Join(before, "\n") != strings.Join(rest, "\n") || len(before) != len(rest) {
			t.Fatalf("other lines changed: %q -> %q", before, rest)
		}
	})
}
