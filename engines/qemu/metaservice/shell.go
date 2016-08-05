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
	// ShellHandshakeTimeout is the maximum allowed time for websocket handshake
	ShellHandshakeTimeout = 3 * time.Second
	// ShellPingInterval is the time between sending pings
	ShellPingInterval = 5 * time.Second
	// ShellWriteTimeout is the maximum time between successful writes
	ShellWriteTimeout = ShellPingInterval * 2
	// ShellPongTimeout is the maximum time between successful reads
	ShellPongTimeout = ShellPingInterval * 3
	// ShellBlockSize is the maximum number of bytes to send in a single block
	ShellBlockSize = 16 * 1024
	// ShellMaxMessageSize is the maximum message size we will read
	ShellMaxMessageSize = ShellBlockSize + 4*1024
	// ShellMaxPendingBytes is the maximum number of bytes allowed in-flight
	ShellMaxPendingBytes = 4 * ShellBlockSize
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
	tellOut := make(chan int, 10)
	tellErr := make(chan int, 10)
	stdout, stdoutWriter := ioext.AsyncPipe(ShellMaxPendingBytes, tellOut)
	stderr, stderrWriter := ioext.AsyncPipe(ShellMaxPendingBytes, tellErr)
	stdinReader.Unblock(ShellMaxPendingBytes)

	s := &shell{
		ws:           ws,
		stdin:        stdin,
		stdout:       stdout,
		stderr:       stderr,
		stdinReader:  stdinReader,
		stdoutWriter: stdoutWriter,
		stderrWriter: stderrWriter,
	}

	ws.SetReadLimit(ShellMaxMessageSize)
	ws.SetReadDeadline(time.Now().Add(ShellPongTimeout))
	ws.SetPongHandler(s.pongHandler)

	go s.writeMessages()
	go s.readMessages()
	go s.sendPings()
	go s.sendAck(StreamStdout, tellOut)
	go s.sendAck(StreamStderr, tellErr)

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
	s.ws.SetWriteDeadline(time.Now().Add(ShellWriteTimeout))
	err := s.ws.WriteMessage(websocket.BinaryMessage, message)
	s.mWrite.Unlock()

	if err != nil {
		s.resolve.Do(func() {
			debug("Resolving internal error: Failed to send message, error: %s", err)
			s.success = false
			s.err = engines.ErrNonFatalInternalError
			s.dispose()
		})
	}
}

func (s *shell) sendPings() {
	for {
		// Sleep for ping interval time
		time.Sleep(ShellPingInterval)

		// Write a ping message, and reset the write deadline
		s.mWrite.Lock()
		s.ws.SetWriteDeadline(time.Now().Add(ShellWriteTimeout))
		err := s.ws.WriteMessage(websocket.PingMessage, []byte{})
		s.mWrite.Unlock()

		// If there is an error we resolve with internal error
		if err != nil {
			s.resolve.Do(func() {
				debug("Resolving with internal-error, failed to send ping, error: %s", err)
				s.success = false
				s.err = engines.ErrNonFatalInternalError
				s.dispose()
			})
			return
		}
	}
}

func (s *shell) sendAck(streamID byte, tell <-chan int) {
	// reserve a buffer for sending acknowledgments
	ack := make([]byte, 2+4)
	ack[0] = MessageTypeAck
	var size int64

	for n := range tell {
		// Merge in as many tell message as is pending
		N := n
		for n > 0 {
			select {
			case n = <-tell:
				N += n
			default:
				n = 0
			}
		}
		// Record the size for logging
		size += int64(N)

		// Send an acknowledgment message (this is for congestion control)
		ack[1] = streamID
		binary.BigEndian.PutUint32(ack[2:], uint32(N))
		s.send(ack)
	}
	debug("Final ack for streamID: %d sent, size: %d", streamID, size)
}

func (s *shell) pongHandler(string) error {
	// Reset the read deadline
	s.ws.SetReadDeadline(time.Now().Add(ShellPongTimeout))
	return nil
}

func (s *shell) writeMessages() {
	m := make([]byte, 2+ShellBlockSize)
	m[0] = MessageTypeData
	m[1] = StreamStdin
	var size int64

	for {
		n, err := s.stdinReader.Read(m[2:])
		size += int64(n)

		// Send payload if more than zero (zero payload indicates end of stream)
		if n > 0 {
			s.send(m[:2+n])
		}

		// If EOF, then we send an empty payload to signal this
		if err == io.EOF {
			debug("Reached EOF of stdin, size: %d", size)
			s.send(m[:2])
			return
		}

		if err != nil && err != io.EOF {
			// If we fail to read from stdin, then we cleanup
			s.resolve.Do(func() {
				debug("Resolving internal error: Failed to read stdin, error: %s", err)
				s.success = false
				s.err = engines.ErrNonFatalInternalError
				s.dispose()
			})
			return
		}
	}
}

func (s *shell) readMessages() {
	for {
		t, m, err := s.ws.ReadMessage()
		if err != nil {
			s.resolve.Do(func() {
				debug("Resolving internal error: Failed to read message, error: %s", err)
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
			if mStream == StreamStdout {
				if len(mPayload) > 0 {
					_, err = s.stdoutWriter.Write(mPayload)
				} else {
					err = s.stdoutWriter.Close()
				}
			}
			if mStream == StreamStderr {
				if len(mPayload) > 0 {
					_, err = s.stderrWriter.Write(mPayload)
				} else {
					err = s.stderrWriter.Close()
				}
			}

			// If there was an error writing to output stream we close with error
			if err != nil {
				s.resolve.Do(func() {
					debug("Resolving internal error: Failed to write streamID: %d, error: %s", mStream, err)
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
				debug("Resolving due to Exit message, success: %v", s.success)

				s.mWrite.Lock()
				s.ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				s.mWrite.Unlock()
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
		debug("Resolving by aborting shell")

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
