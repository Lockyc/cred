package store

import (
	"os"
	"path/filepath"
	"strings"
)

// WriteFile places value at path as the file's entire contents, with a single
// trailing newline so the file is well-formed for `cat` and for tools that
// expect a text file.
//
// Unlike SetEnvKey, WriteFile always enforces the requested mode, even when
// path already exists: the whole point of a standalone destination is that
// the caller picked its mode on purpose. It goes through writeFileAtomic with
// created=true for exactly that reason — a temp file created 0600, chmod'd to
// mode, then renamed onto path, so there is never a window where path itself
// is wider than intended.
func WriteFile(path, value string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return writeFileAtomic(path, []byte(value+"\n"), mode, true)
}

// ReadFile returns the credential stored at path, without the trailing
// newline WriteFile adds.
func ReadFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(b), "\r\n"), nil
}
