package interactive

import (
	"encoding/binary"
	"io"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/websocket"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

// ShellHandler handles a websocket and exposes a reader for stdin, and writers
// for piping out stdout and stderr.
type ShellHandler struct {
	log           *logrus.Entry
	ws            *websocket.Conn
	mWrite        sync.Mutex
	stdin         io.ReadCloser
	stdout        io.WriteCloser
	stderr        io.WriteCloser
	stdinWriter   io.WriteCloser
	stdoutReader  *ioext.PipeReader
	stderrReader  *ioext.PipeReader
	streamingDone sync.WaitGroup // Done when stdout/stderr are done streaming
	resolve       atomics.Once   // wrap calls to abortFunc and success
	abortFunc     func() error
	success       bool
	tellIn        <-chan int
	setSizeFunc   SetSizeFunc
}

// NewShellHandler returns a new ShellHandler structure for that can
// serve/expose a shell over a websocket.
func NewShellHandler(ws *websocket.Conn, log *logrus.Entry) *ShellHandler {
	tellIn := make(chan int, 10)
	stdin, stdinWriter := ioext.AsyncPipe(ShellMaxPendingBytes, tellIn)
	stdoutReader, stdout := ioext.BlockedPipe()
	stderrReader, stderr := ioext.BlockedPipe()
	stdoutReader.Unblock(ShellMaxPendingBytes)
	stderrReader.Unblock(ShellMaxPendingBytes)

	s := &ShellHandler{
		log:          log,
		ws:           ws,
		stdin:        stdin,
		stdout:       stdout,
		stderr:       stderr,
		stdinWriter:  stdinWriter,
		stdoutReader: stdoutReader,
		stderrReader: stderrReader,
		tellIn:       tellIn,
	}

	ws.SetReadLimit(ShellMaxMessageSize)

	return s
}

// A SetSizeFunc can set the set the TTY size of a ter
type SetSizeFunc func(columns, rows uint16) error

// Communicate starts receiving and sending data to/from the exposed pipes.
// Caller provides an optional function setSize for changing the terminal size,
// and a function abort that will be called to kill the underlying shell.
func (s *ShellHandler) Communicate(setSize SetSizeFunc, abort func() error) {
	s.abortFunc = abort
	s.setSizeFunc = setSize

	s.ws.SetReadDeadline(time.Now().Add(ShellPongTimeout))
	s.ws.SetPongHandler(s.pongHandler)

	go s.sendPings()
	go s.waitForSuccess()

	s.streamingDone.Add(2)
	go s.transmitStream(s.stdoutReader, StreamStdout)
	go s.transmitStream(s.stderrReader, StreamStderr)

	go s.readMessages()
	go s.sendAcks()
}

// StdinPipe returns the stdin stream
func (s *ShellHandler) StdinPipe() io.ReadCloser {
	return s.stdin
}

// StdoutPipe returns the stdout stream
func (s *ShellHandler) StdoutPipe() io.WriteCloser {
	return s.stdout
}

// StderrPipe returns the stderr stream
func (s *ShellHandler) StderrPipe() io.WriteCloser {
	return s.stderr
}

// Terminated tells that the shell has been terminated
func (s *ShellHandler) Terminated(success bool) {
	debug("Terminated, success = %t", success)

	// Close pipes
	s.StdoutPipe().Close()
	s.StderrPipe().Close()

	// Wait for stdout/stderr to finish streaming, resolving the shell before
	// all the output from the shell has been read doesn't make any sense.
	debug("Waiting for stdout/stderr to finish")
	s.streamingDone.Wait()

	s.resolve.Do(func() {
		debug("Resolving the shell using Terminated()")
		s.success = success
	})
}

func (s *ShellHandler) abort() {
	debug("Trying to abort (if not already resolved)")
	s.resolve.Do(func() {
		s.log.Error("Resolving the shell using abort()")
		if s.abortFunc != nil {
			s.abortFunc()
		}
		s.success = false
	})
}

func (s *ShellHandler) send(message []byte) {
	// Write message and ensure we reset the write deadline
	s.mWrite.Lock()
	s.ws.SetWriteDeadline(time.Now().Add(ShellWriteTimeout))
	err := s.ws.WriteMessage(websocket.BinaryMessage, message)
	s.mWrite.Unlock()

	if err != nil {
		s.log.Error("Failed to send message, error: ", err)
		s.abort()
	}
}

func (s *ShellHandler) sendPings() {
	for {
		// Sleep for ping interval time
		time.Sleep(ShellPingInterval)

		// Write a ping message, and reset the write deadline
		s.mWrite.Lock()
		debug("Sending ping")
		s.ws.SetWriteDeadline(time.Now().Add(ShellWriteTimeout))
		err := s.ws.WriteMessage(websocket.PingMessage, []byte{})
		s.mWrite.Unlock()

		// If there is an error we resolve with internal error
		if err != nil {
			// This is expected when close is called, it's how this for-loop is broken
			if err == websocket.ErrCloseSent {
				s.abort() // don't log, we probably don't have to call abort() either
				return
			}

			s.log.Error("Failed to send ping, error: ", err)
			s.abort()
			return
		}
	}
}

func (s *ShellHandler) pongHandler(string) error {
	// Reset the read deadline
	s.ws.SetReadDeadline(time.Now().Add(ShellPongTimeout))
	return nil
}

// waitForSuccess will send the exit message when resolved
func (s *ShellHandler) waitForSuccess() {
	// Wait for the shell to be resolved
	s.resolve.Wait()

	debug("shell exited, success = %t", s.success)
	var result byte
	if s.success {
		result = 0
	} else {
		result = 1
	}

	// Lock here instead of using s.send because we want to close after writing
	s.mWrite.Lock()
	defer s.mWrite.Unlock()

	s.ws.SetWriteDeadline(time.Now().Add(ShellWriteTimeout))
	err := s.ws.WriteMessage(websocket.BinaryMessage, []byte{
		MessageTypeExit, result,
	})
	if err != nil {
		s.log.Error("Failed to send 'Exit' message, error: ", err)
	}

	// Close the connection gracefully, We do this because closing the websocket
	// may cause server ping/pongs or acknowledgment messages to fail, so it
	// can't process outstanding messages.
	s.ws.WriteMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
	)

	// Close all streams (in case there's any go-routines blocked on them)
	s.stdinWriter.Close()
	s.stdoutReader.Close()
	s.stderrReader.Close()
}

func (s *ShellHandler) transmitStream(r io.Reader, streamID byte) {
	m := make([]byte, 2+ShellBlockSize)
	m[0] = MessageTypeData
	m[1] = streamID
	var size int64
	for {
		n, err := r.Read(m[2:])
		size += int64(n)

		// Send payload if more than zero (zero payload indicates end of stream)
		if n > 0 {
			s.send(m[:2+n])
		}

		// If EOF, then we send an empty payload to signal this
		if err == io.EOF {
			debug("Reached EOF for streamID: %s size: %d", streamID, size)
			s.send(m[:2])
			// We're done streaming, signal this so an Exit message can be sent.
			s.streamingDone.Done()
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

func (s *ShellHandler) readMessages() {
	for {
		t, m, err := s.ws.ReadMessage()
		if err != nil {
			// This is expected to happen when the loop breaks
			if e, ok := err.(*websocket.CloseError); ok && e.Code == websocket.CloseNormalClosure {
				debug("Websocket closed normally error: ", err)
			} else {
				s.log.Error("Failed to read message from websocket, error: ", err)
			}
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
		if mType == MessageTypeData && len(mData) > 0 {
			// Find [stream] and [payload]
			mStream := mData[0]
			mPayload := mData[1:]

			// Write payload or close stream if payload is zero length
			var err error
			if mStream == StreamStdin {
				if len(mPayload) > 0 {
					_, err = s.stdinWriter.Write(mPayload)
				} else {
					err = s.stdinWriter.Close()
				}
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
		if mType == MessageTypeAck && len(mData) == 5 {
			mStream := mData[0]
			n := binary.BigEndian.Uint32(mData[1:])
			if mStream == StreamStdout {
				s.stdoutReader.Unblock(int64(n))
			}
			if mStream == StreamStderr {
				s.stderrReader.Unblock(int64(n))
			}
		}

		// If we get a size message, we call the set size function
		if mType == MessageTypeSize && len(mData) == 4 {
			cols := binary.BigEndian.Uint16(mData[0:])
			rows := binary.BigEndian.Uint16(mData[2:])
			if s.setSizeFunc != nil {
				s.setSizeFunc(cols, rows)
			}
		}

		// If we get an abort message, we call the abort function
		if mType == MessageTypeAbort && len(mData) == 0 {
			s.resolve.Do(func() {
				debug("aborting the shell because of 'abort' message")
				if s.abortFunc != nil {
					s.abortFunc()
				}
				s.success = false
			})
			return
		}
	}
}

func (s *ShellHandler) sendAcks() {
	// reserve a buffer for sending acknowledgments
	ack := make([]byte, 2+4)
	ack[0] = MessageTypeAck
	ack[1] = StreamStdin
	var size int64

	for n := range s.tellIn {
		// Merge in as many tell message as is pending
		N := n
		for n > 0 {
			select {
			case n = <-s.tellIn:
				N += n
			default:
				n = 0
			}
		}
		// Record the size for logging
		size += int64(N)

		// Send an acknowledgment message (this is for congestion control)
		binary.BigEndian.PutUint32(ack[2:], uint32(N))
		s.send(ack)
	}
	debug("Final ack for stdin sent, size: %d", size)
}
