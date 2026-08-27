package store

import (
	"os"
	"path/filepath"
)

// writeFileAtomic writes data to path through a same-directory temp file
// plus rename, so a crash mid-write leaves the original file intact rather
// than truncated. The temp file is flushed to stable storage before it's
// closed and renamed, so a crash immediately after doesn't leave a
// zero-length file in path's place. Shared by envfile.go's SetEnvKey and
// RemoveEnvKey and by file.go's WriteFile — every writer in this package
// goes through here.
//
// mode applies only when created is true (the file is being written for the
// first time). Otherwise the file at path keeps the mode it already has.
func writeFileAtomic(path string, data []byte, mode os.FileMode, created bool) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".cred-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	perm := mode
	if !created {
		fi, statErr := os.Stat(path)
		if statErr != nil {
			return statErr
		}
		perm = fi.Mode().Perm()
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
