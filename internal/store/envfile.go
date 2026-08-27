package store

import (
	"bufio"
	"bytes"
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

// newScanner returns a line scanner with its buffer cap raised above the
// bufio.Scanner default of 64 KB (a hand-edited .env can legitimately hold a
// long value, e.g. a PEM blob or JWT on one line) so a long line surfaces via
// Err() instead of Scan() silently returning false partway through the file.
func newScanner(b []byte) *bufio.Scanner {
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	return sc
}

// detectEOL reports the line ending already dominant in b. The dominant
// ending is preserved on rewrite; a file with mixed CRLF and LF endings is
// normalised to whichever is dominant, so the other lines' endings do
// change in that case. A file with no newline at all (including one that
// doesn't exist yet) uses "\n".
func detectEOL(b []byte) string {
	crlf := bytes.Count(b, []byte("\r\n"))
	lf := bytes.Count(b, []byte("\n")) - crlf
	if crlf > lf {
		return "\r\n"
	}
	return "\n"
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
	eol := "\n"
	var b strings.Builder
	replaced := false
	if !created {
		eol = detectEOL(existing)
		re := keyLine(key)
		matches := 0
		sc := newScanner(existing)
		for sc.Scan() {
			if re.MatchString(sc.Text()) {
				matches++
				if !replaced {
					b.WriteString(assignment + eol)
					replaced = true
				}
				continue
			}
			b.WriteString(sc.Text() + eol)
		}
		if err := sc.Err(); err != nil {
			return err
		}
		// Refuse rather than guess: .env loaders don't agree on which
		// occurrence of a duplicate key wins, so writing into one
		// occurrence risks silently updating a value the loader ignores.
		// Nothing is written to disk in this case.
		if matches > 1 {
			return fmt.Errorf("key %q appears %d times in %s; remove the duplicate before writing to it — which occurrence a .env loader honors is not portable", key, matches, path)
		}
	}
	if !replaced {
		b.WriteString(assignment + eol)
	}

	return writeFileAtomic(path, []byte(b.String()), mode, created)
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
	sc := newScanner(b)
	matches := 0
	var value string
	for sc.Scan() {
		if re.MatchString(sc.Text()) {
			matches++
			_, v, _ := strings.Cut(sc.Text(), "=")
			value = v
		}
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	switch matches {
	case 0:
		return "", fmt.Errorf("key %q is not set in %s", key, path)
	case 1:
		return unquoteEnv(strings.TrimSpace(value)), nil
	default:
		// Same refusal as SetEnvKey, and for the same reason: which
		// occurrence a .env loader honors is not portable, so reading one
		// occurrence risks reporting a value the loader doesn't actually use.
		return "", fmt.Errorf("key %q appears %d times in %s; remove the duplicate before reading it — which occurrence a .env loader honors is not portable", key, matches, path)
	}
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

// HasEnvKey reports whether key has at least one assignment in path, using
// the same matching as SetEnvKey and GetEnvKey. Unlike GetEnvKey, a duplicate
// key is not an error here — RemoveEnvKey's contract is to drop every
// occurrence, so a caller checking "is there anything to remove" needs
// presence, not a single authoritative value.
func HasEnvKey(path, key string) (bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	re := keyLine(key)
	sc := newScanner(b)
	for sc.Scan() {
		if re.MatchString(sc.Text()) {
			return true, nil
		}
	}
	if err := sc.Err(); err != nil {
		return false, err
	}
	return false, nil
}

// RemoveEnvKey drops key's line from path, leaving the rest untouched. Unlike
// SetEnvKey and GetEnvKey, it deletes every occurrence of a duplicate key
// rather than refusing: that outcome is unambiguous, and it's the user's
// escape hatch out of a duplicate-key file the other two functions won't
// touch.
func RemoveEnvKey(path, key string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	eol := detectEOL(b)
	re := keyLine(key)
	var out strings.Builder
	sc := newScanner(b)
	for sc.Scan() {
		if re.MatchString(sc.Text()) {
			continue
		}
		out.WriteString(sc.Text() + eol)
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return writeFileAtomic(path, []byte(out.String()), 0, false)
}
