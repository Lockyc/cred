//go:build linux

package secret

import "golang.org/x/sys/unix"

// flushInput discards the terminal's pending input (see FromTTY).
func flushInput(fd int) { _ = unix.IoctlSetInt(fd, unix.TCFLSH, unix.TCIFLUSH) }
