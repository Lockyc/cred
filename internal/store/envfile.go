package store

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// A .env assignment: optional leading whitespace, an optional `export `, the
// key, then `=`. Anchored on the full key so API_KEY never matches MY_API_KEY.
func keyLine(key string) *regexp.Regexp {
	return regexp.MustCompile(`^\s*(export\s+)?` + regexp.QuoteMeta(key) + `=`)
}

// SetEnvKey replaces key's line in path, or appends it. Comments, ordering, and
// every other key are preserved: a .env is hand-edited config, not a generated
// file, and rewriting it wholesale destroys the parts a person put there.
//
// mode applies only when the file is created. An existing file keeps the mode
// it already has — the file may legitimately be 644 (a .env.example-shaped file
// under version control), and silently tightening it would break other readers.
func SetEnvKey(path, key, value string, mode os.FileMode) error {
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("value contains a newline, which no .env loader reads back portably")
	}

	existing, err := os.ReadFile(path)
	created := false
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		created = true
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
	}

	assignment := key + "=" + quoteEnv(value)
	var b strings.Builder
	replaced := false
	if !created {
		re := keyLine(key)
		sc := bufio.NewScanner(strings.NewReader(string(existing)))
		sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
		for sc.Scan() {
			if !replaced && re.MatchString(sc.Text()) {
				b.WriteString(assignment + "\n")
				replaced = true
				continue
			}
			b.WriteString(sc.Text() + "\n")
		}
		if err := sc.Err(); err != nil {
			return err
		}
	}
	if !replaced {
		b.WriteString(assignment + "\n")
	}

	// Write through a same-directory temp file and rename, so a crash mid-write
	// leaves the original .env intact rather than truncated.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".env.cred-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.WriteString(b.String()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	perm := mode
	if !created {
		if fi, statErr := os.Stat(path); statErr == nil {
			perm = fi.Mode().Perm()
		}
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// quoteEnv quotes only when the value would otherwise be misread. An unquoted
// token — the overwhelmingly common case for an API key — stays unquoted, so
// cred's output looks like what a person would have typed.
func quoteEnv(v string) string {
	if v == "" {
		return `''`
	}
	if !strings.ContainsAny(v, " \t\"'#=$`\\") {
		return v
	}
	if !strings.Contains(v, "'") {
		return "'" + v + "'"
	}
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "$", `\$`, "`", "\\`")
	return `"` + r.Replace(v) + `"`
}

// GetEnvKey returns the unquoted value of key in path.
func GetEnvKey(path, key string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	re := keyLine(key)
	sc := bufio.NewScanner(strings.NewReader(string(b)))
	for sc.Scan() {
		if re.MatchString(sc.Text()) {
			_, v, _ := strings.Cut(sc.Text(), "=")
			return unquoteEnv(strings.TrimSpace(v)), nil
		}
	}
	return "", fmt.Errorf("key %q is not set in %s", key, path)
}

func unquoteEnv(v string) string {
	if len(v) >= 2 && v[0] == '\'' && v[len(v)-1] == '\'' {
		return v[1 : len(v)-1]
	}
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		r := strings.NewReplacer(`\"`, `"`, `\$`, "$", "\\`", "`", `\\`, `\`)
		return r.Replace(v[1 : len(v)-1])
	}
	return v
}

// RemoveEnvKey drops key's line from path, leaving the rest untouched.
func RemoveEnvKey(path, key string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	re := keyLine(key)
	var out strings.Builder
	sc := bufio.NewScanner(strings.NewReader(string(b)))
	for sc.Scan() {
		if re.MatchString(sc.Text()) {
			continue
		}
		out.WriteString(sc.Text() + "\n")
	}
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(out.String()), fi.Mode().Perm())
}
