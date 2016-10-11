//+build linux darwin dragonfly freebsd netbsd openbsd

package ptysize

import (
	"os"
	"syscall"
	"unsafe"
)

type winSize struct {
	Rows   uint16
	Cols   uint16
	xPixel uint16 // unused
	yPixel uint16 // unused
}

// Supported is true, if Set is supported
const Supported = true

// Set will set terminal size on pty, returns engines.ErrFeatureNotSupported on
// unsupported platforms.
func Set(pty *os.File, cols, rows uint16) error {
	var size winSize
	size.Rows = rows
	size.Cols = cols
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL, pty.Fd(),
		syscall.TIOCSWINSZ, uintptr(unsafe.Pointer(&size)),
	)
	if errno != 0 {
		return errno
	}
	return nil
}
