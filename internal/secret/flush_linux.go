//go:build linux

package secret

import "golang.org/x/sys/unix"

// flushInput discards the terminal's pending input (see FromTTY).
func flushInput(fd int) { _ = unix.IoctlSetInt(fd, unix.TCFLSH, unix.TCIFLUSH) }

// pendingInput returns the number of bytes queued for reading on fd
// (TIOCINQ is Linux's FIONREAD).
func pendingInput(fd int) (int, error) { return unix.IoctlGetInt(fd, unix.TIOCINQ) }
