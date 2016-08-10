package interactive

import (
	"net/http"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/websocket"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

const (
	// ShellHandshakeTimeout is the maximum allowed time for websocket handshake
	ShellHandshakeTimeout = 30 * time.Second
	// ShellPingInterval is the time between sending pings
	ShellPingInterval = 15 * time.Second
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

// A ShellServer implements http.Handler and upgrades connections to websockets,
// creates a shell and connects the websocket to the shell.
type ShellServer struct {
	m             sync.Mutex
	c             sync.Cond
	newShell      func() (engines.Shell, error)
	done          chan struct{}
	refCount      int
	instanceCount int
	log           *logrus.Entry
}

// NewShellServer returns a new ShellServer which creates shells using the
// newShell function.
func NewShellServer(newShell func() (engines.Shell, error), log *logrus.Entry) *ShellServer {
	s := &ShellServer{
		newShell: newShell,
		done:     make(chan struct{}),
		log:      log,
	}
	s.c.L = &s.m
	return s
}

// Wait will wait for all active shells to be done and return
func (s *ShellServer) Wait() {
	s.m.Lock()
	defer s.m.Unlock()
	for s.refCount > 0 {
		s.c.Wait()
	}
}

// WaitAndClose will wait for all active shells to be done and abort creation
// of all new shells atomically.
func (s *ShellServer) WaitAndClose() {
	s.m.Lock()
	defer s.m.Unlock()
	for s.refCount > 0 {
		s.c.Wait()
	}

	// Close done, aborting creation of new shells
	select {
	case <-s.done:
	default:
		close(s.done)
	}
}

// Abort will abort all active interactive shells
func (s *ShellServer) Abort() {
	s.m.Lock()
	defer s.m.Unlock()

	select {
	case <-s.done:
	default:
		close(s.done)
	}
}

var upgrader = websocket.Upgrader{
	HandshakeTimeout: ShellHandshakeTimeout,
	ReadBufferSize:   ShellMaxMessageSize,
	WriteBufferSize:  ShellMaxMessageSize,
}

func (s *ShellServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Quickly check that server haven't been aborted yet
	select {
	case <-s.done:
		w.WriteHeader(http.StatusGone)
		return
	default:
	}

	// Create a new shell, do this before we upgrade so we can return 410 on error
	shell, err := s.newShell()
	if err == engines.ErrSandboxTerminated || err == engines.ErrSandboxAborted {
		w.WriteHeader(http.StatusGone)
		return
	}

	// Upgrade request to a websocket, abort the shell if upgrade fails
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		debug("Failed to upgrade request to websocket, error: %s", err)
		shell.Abort()
		return
	}

	go s.handleShell(ws, shell)
}

func (s *ShellServer) handleShell(ws *websocket.Conn, shell engines.Shell) {
	done := make(chan struct{})

	// Create a shell handler
	s.updateRefCount(1)
	handler := NewShellHandler(ws, s.log.WithField("shell-instance-id", s.nextID()))

	// Connect pipes
	go ioext.CopyAndClose(shell.StdinPipe(), handler.StdinPipe())
	go ioext.CopyAndClose(handler.StdoutPipe(), shell.StdoutPipe())
	go ioext.CopyAndClose(handler.StderrPipe(), shell.StderrPipe())

	// Start streaming
	handler.Communicate(func() {
		shell.Abort()
	})

	// Wait for call to abort all shells
	go func() {
		select {
		case <-s.done:
			shell.Abort()
		case <-done:
		}
	}()

	// Wait for the shell to terminate
	success, _ := shell.Wait()
	handler.Terminated(success)
	s.updateRefCount(-1)

	// Close done so we stop waiting for abort on all shells
	select {
	case <-done:
	default:
		close(done)
	}
}

func (s *ShellServer) updateRefCount(change int) {
	s.m.Lock()
	s.refCount += change
	if s.refCount <= 0 {
		s.c.Broadcast()
	}
	s.m.Unlock()
}

func (s *ShellServer) nextID() int {
	s.m.Lock()
	defer s.m.Unlock()
	ID := s.instanceCount
	s.instanceCount++
	return ID
}
