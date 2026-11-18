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
