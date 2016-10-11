//+build linux darwin dragonfly freebsd netbsd openbsd

package pty

import (
	"os"
	"os/exec"
	"syscall"
	"unsafe"

	"github.com/kr/pty"
)

// Supported is true, if PTY is supported on the given platform
const Supported = true

// PTY holds the pseduo-tty that an exec.Cmd is running in.
type PTY struct {
	f *os.File
}

// Start will create start cmd and return a PTY wrapping it. The PTY implements
// ReadWriteCloser and must be closed when execution is done.
//
// Note: This will overwrite stdio for cmd.
func Start(cmd *exec.Cmd) (*PTY, error) {
	f, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}
	return &PTY{f}, nil
}

func (pty *PTY) Read(p []byte) (n int, err error) {
	return pty.f.Read(p)
}

func (pty *PTY) Write(p []byte) (n int, err error) {
	return pty.f.Write(p)
}

// Close will close the PTY
func (pty *PTY) Close() error {
	return pty.f.Close()
}

// SetSize will set the TTY size, returns engines.ErrFeatureNotSupported on
// unsupported platforms.
func (pty *PTY) SetSize(cols, rows uint16) error {
	var size winSize
	size.Rows = rows
	size.Cols = cols
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL, pty.f.Fd(),
		syscall.TIOCSWINSZ, uintptr(unsafe.Pointer(&size)),
	)
	if errno != 0 {
		return errno
	}
	return nil
}

type winSize struct {
	Rows   uint16
	Cols   uint16
	xPixel uint16 // unused
	yPixel uint16 // unused
}
