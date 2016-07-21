package metaservice

import (
	"encoding/binary"
	"io"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

const (
	pongTimeout         = 30 * time.Second
	writeTimeout        = pongTimeout * 3 / 2
	pingInterval        = 10 * time.Second
	maxMessageSize      = 10 * 1024
	maxOutstandingBytes = 8 * 1024
	readBlockSize       = 4 * 1024
)

type shell struct {
	ws           *websocket.Conn
	mWrite       sync.Mutex
	stdin        io.WriteCloser
	stdout       io.ReadCloser
	stderr       io.ReadCloser
	stdinReader  *ioext.PipeReader
	stdoutWriter io.WriteCloser
	stderrWriter io.WriteCloser
	resolve      atomics.Once // Must wrap access to success/err
	success      bool
	err          error
}

// newShell takes a websocket and creates a shell object implementing the
// engines.Shell interface.
func newShell(ws *websocket.Conn) *shell {
	stdinReader, stdin := ioext.BlockedPipe()
	stdout, stdoutWriter := io.Pipe()
	stderr, stderrWriter := io.Pipe()
	stdinReader.Unblock(maxOutstandingBytes)

	s := &shell{
		ws:           ws,
		stdin:        stdin,
		stdout:       stdout,
		stderr:       stderr,
		stdinReader:  stdinReader,
		stdoutWriter: stdoutWriter,
		stderrWriter: stderrWriter,
	}

	ws.SetReadLimit(maxMessageSize)
	ws.SetReadDeadline(time.Now().Add(pongTimeout))
	ws.SetPongHandler(s.pongHandler)

	go s.writeMessages()
	go s.readMessages()
	go s.sendPings()

	return s
}

func (s *shell) dispose() {
	// Close websocket
	s.ws.Close()

	// Close all streams
	s.stdinReader.Close()
	s.stdoutWriter.Close()
	s.stderrWriter.Close()
}

func (s *shell) send(message []byte) {
	// Write message and ensure we reset the write deadline
	s.mWrite.Lock()
	s.ws.SetWriteDeadline(time.Now().Add(writeTimeout))
	err := s.ws.WriteMessage(websocket.BinaryMessage, message)
	s.mWrite.Unlock()

	if err != nil {
		s.resolve.Do(func() {
			s.success = false
			s.err = engines.ErrNonFatalInternalError
			s.dispose()
		})
	}
}

func (s *shell) sendPings() {
	for {
		// Sleep for ping interval time
		time.Sleep(pingInterval)

		// Write a ping message, and reset the write deadline
		s.mWrite.Lock()
		s.ws.SetWriteDeadline(time.Now().Add(writeTimeout))
		err := s.ws.WriteMessage(websocket.PingMessage, []byte{})
		s.mWrite.Unlock()

		// If there is an error we resolve with internal error
		if err != nil {
			s.resolve.Do(func() {
				s.success = false
				s.err = engines.ErrNonFatalInternalError
				s.dispose()
			})
			return
		}
	}
}

func (s *shell) pongHandler(string) error {
	// Reset the read deadline
	s.ws.SetReadDeadline(time.Now().Add(pongTimeout))
	return nil
}

func (s *shell) writeMessages() {
	m := make([]byte, 2+readBlockSize)
	m[0] = MessageTypeData
	m[1] = StreamStdin
	for {
		n, err := s.stdinReader.Read(m[2:])

		// Send payload if more than zero (zero payload indicates end of stream)
		if n > 0 {
			s.send(m[:2+n])
		}

		// If EOF, then we send an empty payload to signal this
		if err == io.EOF {
			s.send(m[:2])
			return
		}

		if err != nil && err != io.EOF {
			// If we fail to read from stdin, then we cleanup
			s.resolve.Do(func() {
				s.success = false
				s.err = engines.ErrNonFatalInternalError
				s.dispose()
			})
			return
		}
	}
}

func (s *shell) readMessages() {
	// reserve a buffer for sending acknowledgments
	ack := make([]byte, 2+4)
	ack[0] = MessageTypeAck

	for {
		t, m, err := s.ws.ReadMessage()
		if err != nil {
			s.resolve.Do(func() {
				s.success = false
				s.err = engines.ErrNonFatalInternalError
				s.dispose()
			})
			return
		}

		// Skip anything that isn't a binary message
		if t != websocket.BinaryMessage || len(m) == 0 {
			continue
		}

		// Find [type] and [data]
		mType := m[0]
		mData := m[1:]

		// If we get a datatype
		if mType == MessageTypeData && len(mData) > 0 {
			// Find [stream] and [payload]
			mStream := mData[0]
			mPayload := mData[1:]

			// Write payload or close stream if payload is zero length
			var err error
			var n int
			if mStream == StreamStdout {
				if len(mPayload) > 0 {
					n, err = s.stdoutWriter.Write(mPayload)
				} else {
					err = s.stdoutWriter.Close()
				}
			}
			if mStream == StreamStderr {
				if len(mPayload) > 0 {
					n, err = s.stderrWriter.Write(mPayload)
				} else {
					err = s.stderrWriter.Close()
				}
			}

			// If payload was non-zero and successfully written we send an
			// acknowledgment message (this is for congestion control)
			if err == nil && n > 0 {
				ack[1] = mStream
				binary.BigEndian.PutUint32(ack[2:], uint32(n))
				s.send(ack)
			}

			// If there was an error writing to output stream we close with error
			if err != nil {
				s.resolve.Do(func() {
					s.success = false
					s.err = engines.ErrNonFatalInternalError
					s.dispose()
				})
				return
			}
		}

		// If bytes from stdin are acknowleged, then we unblock additional bytes
		if mType == MessageTypeAck && len(mData) == 5 {
			if mData[0] == StreamStdin {
				n := binary.BigEndian.Uint32(mData[1:])
				s.stdinReader.Unblock(int64(n))
			}
		}

		// If we get an exit message, we resolve and close the websocket
		if mType == MessageTypeExit && len(mData) == 1 {
			s.resolve.Do(func() {
				s.success = (mData[0] == 0)
				s.err = engines.ErrShellTerminated
				s.dispose()
			})
			return
		}
	}
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
	s.resolve.Do(func() {
		// Write an abort message
		m := make([]byte, 1)
		m[0] = MessageTypeAbort
		s.send(m)

		// Set success false, err to shell aborted
		s.success = false
		s.err = engines.ErrShellAborted

		// Close the websocket
		s.dispose()
	})

	s.resolve.Wait()
	if s.err == engines.ErrShellAborted {
		return nil
	}
	return s.err
}

func (s *shell) Wait() (bool, error) {
	s.resolve.Wait()
	if s.err == engines.ErrShellTerminated {
		return s.success, nil
	}
	return s.success, s.err
}
