//+build linux darwin dragonfly freebsd netbsd openbsd

package qemuguesttools

import (
	"os"
	"os/exec"
	"syscall"
	"unsafe"

	"github.com/kr/pty"
	"github.com/taskcluster/taskcluster-worker/plugins/interactive"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

func pipePty(cmd *exec.Cmd, handler *interactive.ShellHandler) error {
	// Start the shell as TTY
	f, err := pty.Start(cmd)
	if err == nil {
		// Connect pipes (close stderr as tty only has two streams)
		go ioext.CopyAndClose(f, handler.StdinPipe())
		go ioext.CopyAndClose(handler.StdoutPipe(), f)
		go handler.StderrPipe().Close()

		// Start communication
		handler.Communicate(func(cols, rows uint16) error {
			return setTTYSize(f, cols, rows)
		}, func() error {
			if cmd.Process != nil {
				return cmd.Process.Kill()
			}
			return nil
		})
	}
	// If pty wasn't supported for some reason we fall back to normal execution
	if err == pty.ErrUnsupported {
		return pipeCommand(cmd, handler)
	}
	if err != nil {
		handler.Communicate(nil, func() error {
			// If cmd.Start() failed, then we don't have a process, but we start
			// the communication flow anyways.
			if cmd.Process != nil {
				return cmd.Process.Kill()
			}
			return nil
		})
	}
	return err
}

type winSize struct {
	Rows   uint16
	Cols   uint16
	xPixel uint16 // unused
	yPixel uint16 // unused
}

// setTTYSize will set the terminal size on pty
func setTTYSize(pty *os.File, cols, rows uint16) error {
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
