package qemuguesttools

import (
	"encoding/binary"
	"io"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/websocket"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/metaservice"
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

type shellHandler struct {
	log          *logrus.Entry
	ws           *websocket.Conn
	mWrite       sync.Mutex
	stdin        io.ReadCloser
	stdout       io.WriteCloser
	stderr       io.WriteCloser
	stdinWriter  io.WriteCloser
	stdoutReader *ioext.PipeReader
	stderrReader *ioext.PipeReader
	resolve      atomics.Once // wrap calls to abortFunc and success
	abortFunc    func()
	success      bool
}

// newShellHandler returns a new shellHandler structure for that can
// serve/expose a shell over a websocket.
func newShellHandler(ws *websocket.Conn, log *logrus.Entry) *shellHandler {
	stdin, stdinWriter := io.Pipe()
	stdoutReader, stdout := ioext.BlockedPipe()
	stderrReader, stderr := ioext.BlockedPipe()
	stdoutReader.Unblock(maxOutstandingBytes)
	stderrReader.Unblock(maxOutstandingBytes)

	s := &shellHandler{
		log:          log,
		ws:           ws,
		stdin:        stdin,
		stdout:       stdout,
		stderr:       stderr,
		stdinWriter:  stdinWriter,
		stdoutReader: stdoutReader,
		stderrReader: stderrReader,
	}

	ws.SetReadLimit(maxMessageSize)

	return s
}

// Communicate starts receiving and sending data to/from the exposed pipes.
// Caller provides an abortFunc to be called to abort/kill the underlying shell.
func (s *shellHandler) Communicate(abortFunc func()) {
	s.abortFunc = abortFunc

	s.ws.SetReadDeadline(time.Now().Add(pongTimeout))
	s.ws.SetPongHandler(s.pongHandler)

	go s.sendPings()
	go s.waitForSuccess()

	go s.transmitStream(s.stdoutReader, metaservice.StreamStdout)
	go s.transmitStream(s.stderrReader, metaservice.StreamStderr)

	go s.readMessages()
}

// StdinPipe returns the stdin stream
func (s *shellHandler) StdinPipe() io.ReadCloser {
	return s.stdin
}

// StdoutPipe returns the stdout stream
func (s *shellHandler) StdoutPipe() io.WriteCloser {
	return s.stdout
}

// StderrPipe returns the stderr stream
func (s *shellHandler) StderrPipe() io.WriteCloser {
	return s.stderr
}

// Terminated tells that the shell has been terminated
func (s *shellHandler) Terminated(success bool) {
	s.log.Info("Terminated, success = ", success)
	s.resolve.Do(func() {
		s.log.Info("Resovling the shell using Terminated()")
		s.success = success
	})
}

func (s *shellHandler) abort() {
	s.log.Info("aborting due to error")
	s.resolve.Do(func() {
		s.log.Error("Resolving the shell using abort()")
		if s.abortFunc != nil {
			s.abortFunc()
		}
		s.success = false
	})
}

func (s *shellHandler) send(message []byte) {
	// Write message and ensure we reset the write deadline
	s.mWrite.Lock()
	s.ws.SetWriteDeadline(time.Now().Add(writeTimeout))
	err := s.ws.WriteMessage(websocket.BinaryMessage, message)
	s.mWrite.Unlock()

	if err != nil {
		s.log.Error("Failed to send message, error: ", err)
		s.abort()
	}
}

func (s *shellHandler) sendPings() {
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
			// This is expected when close is called, it's how this for-loop is broken
			s.log.Info("Failed to send ping, error: ", err)
			s.abort()
			return
		}
	}
}

func (s *shellHandler) pongHandler(string) error {
	// Reset the read deadline
	s.ws.SetReadDeadline(time.Now().Add(pongTimeout))
	return nil
}

// waitForSuccess will send the exit message when resolved
func (s *shellHandler) waitForSuccess() {
	// Wait for the shell to be resolved
	s.resolve.Wait()

	s.log.Info("shell exited, success = ", s.success)
	var result byte
	if s.success {
		result = 0
	} else {
		result = 1
	}

	// Lock here instead of using s.send because we want to close after writing
	s.mWrite.Lock()
	defer s.mWrite.Unlock()

	s.ws.SetWriteDeadline(time.Now().Add(writeTimeout))
	err := s.ws.WriteMessage(websocket.BinaryMessage, []byte{
		metaservice.MessageTypeExit, result,
	})
	if err != nil {
		s.log.Error("Failed to send 'Exit' message, error: ", err)
	}

	// Close the websocket
	s.ws.Close()

	// Close all streams (in case there's any go-routines blocked on them)
	s.stdinWriter.Close()
	s.stdoutReader.Close()
	s.stderrReader.Close()
}

func (s *shellHandler) transmitStream(r io.Reader, streamID byte) {
	m := make([]byte, 2+readBlockSize)
	m[0] = metaservice.MessageTypeData
	m[1] = streamID
	for {
		n, err := r.Read(m[2:])

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
			// If we fail to read with some other error we abort
			s.log.Error("Failed to read streamId: ", streamID, " error: ", err)
			s.abort()
			return
		}
	}
}

func (s *shellHandler) readMessages() {
	// reserve a buffer for sending acknowledgments
	ack := make([]byte, 2+4)
	ack[0] = metaservice.MessageTypeAck
	ack[1] = metaservice.StreamStdin

	for {
		t, m, err := s.ws.ReadMessage()
		if err != nil {
			// This is expected to happen when the loop breaks
			s.log.Info("Failed to read message from websocket, error: ", err)
			s.abort()
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
		if mType == metaservice.MessageTypeData && len(mData) > 0 {
			// Find [stream] and [payload]
			mStream := mData[0]
			mPayload := mData[1:]

			// Write payload or close stream if payload is zero length
			var err error
			var n int
			if mStream == metaservice.StreamStdin {
				if len(mPayload) > 0 {
					n, err = s.stdinWriter.Write(mPayload)
				} else {
					err = s.stdinWriter.Close()
				}
			}

			// If payload was non-zero and successfully written we send an
			// acknowledgment message (this is for congestion control)
			if err == nil && n > 0 {
				binary.BigEndian.PutUint32(ack[2:], uint32(n))
				s.send(ack)
			}
			// If there are errors writing to stdin, then we'll abort...
			// The right thing might be to return an error, as in pipe-broken...
			// Maybe one day we can consider this, for now abort seems reasonable.
			if err != nil {
				s.log.Error("Failed to write to stdin, error: ", err)
				s.abort()
				return
			}
		}

		// If bytes from stdout/stderr are acknowleged, then we unblock
		// additional bytes
		if mType == metaservice.MessageTypeAck && len(mData) == 5 {
			n := binary.BigEndian.Uint32(mData[1:])
			if mData[0] == metaservice.StreamStdout {
				s.stdoutReader.Unblock(int64(n))
			}
			if mData[0] == metaservice.StreamStderr {
				s.stderrReader.Unblock(int64(n))
			}
		}

		// If we get an abort message, we call the abort function
		if mType == metaservice.MessageTypeAbort && len(mData) == 0 {
			s.resolve.Do(func() {
				s.log.Info("aborting the shell because of 'abort' message")
				if s.abortFunc != nil {
					s.abortFunc()
				}
				s.success = false
			})
			return
		}
	}
}
