package mockengine

import (
	"fmt"
	"io"
	"io/ioutil"
	"sync"
	"time"

	"github.com/taskcluster/taskcluster-worker/engines"
)

type shell struct {
	sync.Mutex
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
	done   chan struct{}
	abort  error
	result bool
}

func newShell() *shell {
	stdoutReader, stdoutWriter := io.Pipe()
	stderrReader, stderrWriter := io.Pipe()
	stdinReader, stdinWriter := io.Pipe()

	s := &shell{
		stdin:  stdinWriter,
		stdout: stdoutReader,
		stderr: stderrReader,
		done:   make(chan struct{}),
	}

	go func() {
		resultError := error(nil)
		resultValue := true

		// Read stdin
		data, err := ioutil.ReadAll(stdinReader)
		if err != nil {
			resultError = fmt.Errorf("Error reading stdin: %s", err)
		}

		// Execute command
		switch string(data) {
		case "print-hello":
			_, werr := stdoutWriter.Write([]byte("Hello World"))
			if werr != nil && resultError == nil {
				resultError = fmt.Errorf("Error while writing stdout: %s", werr)
			}
			_, werr = stderrWriter.Write([]byte("No error!"))
			if werr != nil && resultError == nil {
				resultError = fmt.Errorf("Error while writing stderr: %s", werr)
			}
		case "exit-false":
			resultValue = false
		case "sleep":
			time.Sleep(time.Second / 10)
		}

		// Close stdout/stderr
		err = stdoutWriter.Close()
		if err != nil && resultError == nil {
			resultError = fmt.Errorf("Failed to close stdout: %s", err)
		}
		err = stderrWriter.Close()
		if err != nil && resultError == nil {
			resultError = fmt.Errorf("Failed to close stderr: %s", err)
		}

		// Lock to avoid racing between <-s.done and close(s.done)
		s.Lock()
		defer s.Unlock()

		select {
		case <-s.done:
			// If already done, we must have been aborted, whatever happened doesn't
			// matter anymore
		default:
			// If not done yet, we set the resultError and resultValue
			// the close the channel to signal that we're done.
			s.abort = resultError
			s.result = resultValue
			close(s.done)
		}
	}()

	return s
}

func (s *shell) SetSize(columns, rows uint16) error {
	return engines.ErrFeatureNotSupported
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

func (s *shell) Abort() error {
	// Lock to avoid racing between <-s.done and close(s.done)
	s.Lock()
	defer s.Unlock()

	select {
	case <-s.done:
		// If already done, we can't abort as the shell has terminated normally
		return engines.ErrShellTerminated
	default:
		// If not done we signal that the terminal is aborted
		s.abort = engines.ErrShellAborted
		s.result = false
		close(s.done)
	}
	return nil
}

func (s *shell) Wait() (bool, error) {
	// Wait for done to be signaled
	<-s.done
	// Now it's safe to read result and error, we'll never change them!
	return s.result, s.abort
}
