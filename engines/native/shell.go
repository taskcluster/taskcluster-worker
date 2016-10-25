package nativeengine

import (
	"bytes"
	"io"
	"io/ioutil"

	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/native/system"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
)

type shell struct {
	process    *system.Process
	isTTY      bool
	stdin      io.WriteCloser
	stdout     io.ReadCloser
	stderr     io.ReadCloser
	resolve    atomics.Once // Guarding result, resultErr and abortErr
	result     bool
	resultErr  error
	abortErr   error
	aborted    atomics.Bool
	terminated atomics.Bool
}

func newShell(s *sandbox, command []string, tty bool) (*shell, error) {
	// Setup some pipes
	pipein, stdin := io.Pipe()
	stdout, pipeout := io.Pipe()
	var stderr io.ReadCloser
	var pipeerr io.WriteCloser
	if !tty {
		stderr, pipeerr = io.Pipe()
	} else {
		// If doing a TTY we merge stderr and stdout, so stderr just becomes an
		// empty stream as far as client is aware
		stderr = ioutil.NopCloser(bytes.NewBuffer(nil))
	}

	process, err := system.StartProcess(system.ProcessOptions{
		Arguments:     command,
		Environment:   s.env,
		WorkingFolder: s.homeFolder.Path(),
		Owner:         s.user,
		Stdin:         pipein,
		Stdout:        pipeout,
		Stderr:        pipeerr,
		TTY:           tty,
	})
	if err != nil {
		return nil, err
	}

	shell := &shell{
		process: process,
		stdin:   stdin,
		stdout:  stdout,
		stderr:  stderr,
	}

	go shell.waitForResult()

	return shell, nil
}

func (s *shell) waitForResult() {
	// wait for process to terminate
	success := s.process.Wait()

	s.resolve.Do(func() {
		s.terminated.Set(true)
		s.result = success
		s.abortErr = engines.ErrShellTerminated
	})
}

func (s *shell) StdinPipe() io.WriteCloser {
	return s.stdin
}

func (s *shell) StdoutPipe() io.ReadCloser {
	return s.stdout
}

func (s *shell) StderrPipe() io.ReadCloser {
	return s.stderr
}

func (s *shell) SetSize(columns, rows uint16) error {
	// Best effort check if we've terminated
	if s.aborted.Get() {
		return engines.ErrSandboxAborted
	}
	if s.terminated.Get() {
		return engines.ErrShellTerminated
	}
	// Feature not supported if not tty
	if s.isTTY {
		s.process.SetSize(columns, rows)
		return nil
	}
	return engines.ErrFeatureNotSupported
}

func (s *shell) Abort() error {
	s.resolve.Do(func() {
		s.aborted.Set(true)
		s.process.Kill()
		s.resultErr = engines.ErrShellAborted
	})
	s.resolve.Wait()
	return s.resultErr
}

func (s *shell) Wait() (bool, error) {
	s.resolve.Wait()
	return s.result, s.resultErr
}
