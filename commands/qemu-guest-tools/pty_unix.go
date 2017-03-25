//+build linux darwin dragonfly freebsd netbsd openbsd

package qemuguesttools

import (
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"github.com/kr/pty"
	"github.com/taskcluster/taskcluster-worker/plugins/interactive"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

func copyCloseDone(w io.WriteCloser, r io.Reader, wg *sync.WaitGroup) {
	ioext.CopyAndClose(w, r)
	wg.Done()
}

func pipePty(cmd *exec.Cmd, handler *interactive.ShellHandler) error {
	// Start the shell as TTY
	f, err := pty.Start(cmd)
	if err != nil {
		// If pty wasn't supported for some reason we fall back to normal execution
		if err == pty.ErrUnsupported {
			return pipeCommand(cmd, handler)
		}

		// If cmd.Start() failed, then we don't have a process, but we start
		// the communication flow anyways.
		handler.Communicate(nil, func() error {
			if cmd.Process != nil {
				return cmd.Process.Kill()
			}
			return nil
		})
		return err
	}

	// Connect pipes (close stderr as tty only has two streams)
	wg := sync.WaitGroup{}
	wg.Add(2)
	go copyCloseDone(f, handler.StdinPipe(), &wg)
	go copyCloseDone(handler.StdoutPipe(), f, &wg)
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

	// If starting the shell didn't fail, then we wait for the shell to terminate
	err = cmd.Wait()
	wg.Wait() // wait for pipes to be copied out before returning
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
