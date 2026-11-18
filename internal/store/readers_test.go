package store

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joho/godotenv"
)

// godotenvMisreads are values whose double-quoted encoding godotenv misreads
// or fails on: each ends in \ or ". Set may refuse them, but anything it
// writes for them must read back.
var godotenvMisreads = map[string]bool{`abc\`: true, `o'clock\`: true, `\"`: true, `it's "q"`: true}

// Every value cred writes reads back unchanged in godotenv, a reader
// quoteEnv promises to agree with.
func TestSetEnvKeyReadsBackInGodotenv(t *testing.T) {
	values := readerCorpus()
	for v := range godotenvMisreads {
		values = append(values, v)
	}
	for _, value := range values {
		p := filepath.Join(t.TempDir(), ".env")
		if err := SetEnvKey(p, "K", value, 0o600); err != nil {
			if godotenvMisreads[value] {
				continue
			}
			t.Fatalf("Set(%q): %v", value, err)
		}
		env, err := godotenv.Read(p)
		if err != nil {
			t.Errorf("godotenv.Read after Set(%q): %v", value, err)
			continue
		}
		if got := env["K"]; got != value {
			t.Errorf("godotenv reads Set(%q) as %q (file %q)", value, got, readEnv(t, p))
		}
	}
}

// godotenvDiffers pins the hand-written lines godotenv reads differently from
// GetEnvKey and `. ./.env`, with godotenv's value: it cuts a bare value at
// the last ` #`, not the first.
var godotenvDiffers = map[string]string{
	"K=a #b #c": "a #b",
}

// godotenv reads every hand-written line the way GetEnvKey does, except the
// pinned ones, so a change in either reader shows up here.
func TestGetEnvKeyAgreesWithGodotenvOnHandWrittenLines(t *testing.T) {
	for line := range trailingCommentCases() {
		p := writeEnv(t, line+"\n")
		got, err := GetEnvKey(p, "K")
		if err != nil {
			t.Fatalf("%q: %v", line, err)
		}
		env, err := godotenv.Read(p)
		if err != nil {
			t.Fatalf("godotenv.Read(%q): %v", line, err)
		}
		want := got
		if pinned, ok := godotenvDiffers[line]; ok {
			want = pinned
		}
		if env["K"] != want {
			t.Errorf("godotenv reads %q as %q, want %q (GetEnvKey reads %q)", line, env["K"], want, got)
		}
	}
}

// pythonDotenv is the python-dotenv release the `just readers` recipe pins.
const pythonDotenv = "python-dotenv==1.2.4"

// pythonDotenvRead returns python-dotenv's reading of the .env at path. It
// runs only under `just readers` (CRED_READERS=1), and skips where uvx is
// absent.
func pythonDotenvRead(t *testing.T, path string) map[string]string {
	t.Helper()
	if os.Getenv("CRED_READERS") == "" {
		t.Skip("python-dotenv runs under `just readers`")
	}
	if _, err := exec.LookPath("uvx"); err != nil {
		t.Skip("uvx not on PATH")
	}
	cmd := exec.Command("uvx", "--with", pythonDotenv, "python", "-c",
		"import sys, json; from dotenv import dotenv_values; json.dump(dotenv_values(sys.argv[1]), sys.stdout)", path)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("python-dotenv on %s: %v", path, err)
	}
	var env map[string]string
	if err := json.Unmarshal(out, &env); err != nil {
		t.Fatalf("python-dotenv output %q: %v", out, err)
	}
	return env
}

// Every value cred writes reads back unchanged in python-dotenv.
func TestSetEnvKeyReadsBackInPythonDotenv(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".env")
	values := readerCorpus()
	for i, value := range values {
		if err := SetEnvKey(p, fmt.Sprintf("K%d", i), value, 0o600); err != nil {
			t.Fatalf("Set(%q): %v", value, err)
		}
	}
	env := pythonDotenvRead(t, p)
	for i, value := range values {
		if got := env[fmt.Sprintf("K%d", i)]; got != value {
			t.Errorf("python-dotenv reads Set(%q) as %q", value, got)
		}
	}
}

// python-dotenv reads every hand-written line the way GetEnvKey does.
func TestGetEnvKeyAgreesWithPythonDotenvOnHandWrittenLines(t *testing.T) {
	var body strings.Builder
	var lines []string
	for line := range trailingCommentCases() {
		lines = append(lines, line)
	}
	for i, line := range lines {
		body.WriteString(strings.Replace(line, "K", fmt.Sprintf("K%d", i), 1) + "\n")
	}
	p := writeEnv(t, body.String())
	env := pythonDotenvRead(t, p)
	for i, line := range lines {
		key := fmt.Sprintf("K%d", i)
		got, err := GetEnvKey(p, key)
		if err != nil {
			t.Fatalf("%q: %v", line, err)
		}
		if env[key] != got {
			t.Errorf("python-dotenv reads %q as %q, GetEnvKey as %q", line, env[key], got)
		}
	}
}
