package store

import (
	"path/filepath"
	"testing"

	"github.com/joho/godotenv"
)

// Every value cred writes reads back unchanged in godotenv, a reader
// quoteEnv promises to agree with.
func TestSetEnvKeyReadsBackInGodotenv(t *testing.T) {
	for _, value := range readerCorpus() {
		p := filepath.Join(t.TempDir(), ".env")
		if err := SetEnvKey(p, "K", value, 0o600); err != nil {
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
	"K=a #b #c": "a #b",}

// godotenv reads every hand-written line the way GetEnvKey does, except the
// pinned ones, so a change in either reader shows up here.
func TestGetEnvKeyAgreesWithGodotenvOnHandWrittenLines(t *testing.T) {
	for line, want := range trailingCommentCases() {
		p := writeEnv(t, line+"\n")
		got, err := GetEnvKey(p, "K")
		if err != nil {
			t.Fatalf("%q: %v", line, err)
		}
		env, err := godotenv.Read(p)
		if err != nil {
			t.Fatalf("godotenv.Read(%q): %v", line, err)
		}
		if pinned, ok := godotenvDiffers[line]; ok {
			want = pinned
		}
		if env["K"] != want {
			t.Errorf("godotenv reads %q as %q, want %q (GetEnvKey reads %q)", line, env["K"], want, got)
		}
	}
}
