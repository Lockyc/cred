//go:build darwin || freebsd || netbsd || openbsd || dragonfly

package secret

import "golang.org/x/sys/unix"

// fread is <sys/fcntl.h>'s FREAD: TIOCFLUSH's argument selecting the input queue.
const fread = 0x1

// flushInput discards the terminal's pending input (see FromTTY).
func flushInput(fd int) { _ = unix.IoctlSetPointerInt(fd, unix.TIOCFLUSH, fread) }

// fionread is <sys/filio.h>'s FIONREAD, _IOR('f', 127, int), which x/sys/unix
// does not export for these systems.
const fionread = 0x4004667f

// pendingInput returns the number of bytes queued for reading on fd.
func pendingInput(fd int) (int, error) { return unix.IoctlGetInt(fd, fionread) }
