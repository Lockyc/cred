//go:build darwin || freebsd || netbsd || openbsd || dragonfly

package secret

import "golang.org/x/sys/unix"

// fread is <sys/fcntl.h>'s FREAD: TIOCFLUSH's argument selecting the input queue.
const fread = 0x1

// flushInput discards the terminal's pending input (see FromTTY).
func flushInput(fd int) { _ = unix.IoctlSetPointerInt(fd, unix.TIOCFLUSH, fread) }
