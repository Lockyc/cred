//go:build unix

package store

import (
	"os"
	"syscall"
)

// keepOwner gives the file at tmp the owner and group of fi, the file it is
// about to replace.
func keepOwner(tmp string, fi os.FileInfo) error {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	return os.Chown(tmp, int(st.Uid), int(st.Gid))
}
