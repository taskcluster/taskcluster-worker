//+build !linux,!darwin,!dragonfly,!freebsd,!netbsd,!openbsd

package pty

import (
	"io"
	"os/exec"

	"github.com/taskcluster/taskcluster-worker/engines"
)

// Supported is true, if PTY is supported on the given platform
const Supported = false

// PTY holds the pseduo-tty that an exec.Cmd is running in.
type PTY struct {
	in  io.WriteCloser
	out io.ReadCloser
}

// Start will create start cmd and return a PTY wrapping it. The PTY implements
// ReadWriteCloser and must be closed when execution is done.
//
// Note: This will overwrite stdio for cmd.
func Start(cmd *exec.Cmd) (*PTY, error) {
	stdin, in := io.Pipe()
	out, outWriter := io.Pipe()
	cmd.Stdin = stdin
	cmd.Stdout = outWriter
	cmd.Stderr = outWriter
	err := cmd.Start()
	if err != nil {
		return nil, err
	}
	return &PTY{in, out}, nil
}

func (pty *PTY) Read(p []byte) (n int, err error) {
	return pty.out.Read(p)
}

func (pty *PTY) Write(p []byte) (n int, err error) {
	return pty.in.Write(p)
}

// Close will close the PTY
func (pty *PTY) Close() error {
	pty.in.Close()
	return pty.out.Close()
}

// SetSize will set the TTY size, returns engines.ErrFeatureNotSupported on
// unsupported platforms.
func (pty *PTY) SetSize(cols, rows uint16) error {
	return engines.ErrFeatureNotSupported
}
