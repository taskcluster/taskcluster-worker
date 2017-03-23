package interactive

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins/interactive/shellconsts"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

// ShellFactory is a function that can make a shell
type ShellFactory func(command []string, tty bool) (engines.Shell, error)

// A ShellServer implements http.Handler and upgrades connections to websockets,
// creates a shell and connects the websocket to the shell.
type ShellServer struct {
	m             sync.Mutex
	c             sync.Cond
	makeShell     ShellFactory
	done          chan struct{}
	refCount      int
	instanceCount int
	monitor       runtime.Monitor
}

// NewShellServer returns a new ShellServer which creates shells using the
// makeShell function.
func NewShellServer(makeShell ShellFactory, monitor runtime.Monitor) *ShellServer {
	s := &ShellServer{
		makeShell: makeShell,
		done:      make(chan struct{}),
		monitor:   monitor,
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
	HandshakeTimeout: shellconsts.ShellHandshakeTimeout,
	ReadBufferSize:   shellconsts.ShellMaxMessageSize,
	WriteBufferSize:  shellconsts.ShellMaxMessageSize,
	CheckOrigin:      func(_ *http.Request) bool { return true },
}

func (s *ShellServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Quickly check that server haven't been aborted yet
	select {
	case <-s.done:
		setCORS(w)
		w.WriteHeader(http.StatusGone)
		return
	default:
	}

	// Get command and tty from query-string
	qs := r.URL.Query()
	command := qs["command"]
	tty := strings.ToLower(qs.Get("tty")) == "true"

	// Create a new shell, do this before we upgrade so we can return 410 on error
	shell, err := s.makeShell(command, tty)
	if err == engines.ErrSandboxTerminated || err == engines.ErrSandboxAborted {
		setCORS(w)
		w.WriteHeader(http.StatusGone)
		return
	}
	if err != nil {
		setCORS(w)
		w.WriteHeader(http.StatusInternalServerError)
		debug("Failed to create shell, error: %s", err)
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

func copyCloseDone(w io.WriteCloser, r io.Reader, wg *sync.WaitGroup) {
	ioext.CopyAndClose(w, r)
	wg.Done()
}

func (s *ShellServer) handleShell(ws *websocket.Conn, shell engines.Shell) {
	done := make(chan struct{})

	// Create a shell handler
	s.updateRefCount(1)
	handler := NewShellHandler(ws, s.monitor.WithTag("shell-instance-id", fmt.Sprintf("%d", s.nextID())))

	// Connect pipes
	wg := sync.WaitGroup{}
	wg.Add(3)
	go copyCloseDone(shell.StdinPipe(), handler.StdinPipe(), &wg)
	go copyCloseDone(handler.StdoutPipe(), shell.StdoutPipe(), &wg)
	go copyCloseDone(handler.StderrPipe(), shell.StderrPipe(), &wg)

	// Start streaming
	handler.Communicate(shell.SetSize, shell.Abort)

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
	wg.Wait() // Wait for pipes to be copied before terminating
	handler.Terminated(success)
	s.updateRefCount(-1)

	// Close done so we stop waiting for abort on all shells
	close(done)
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
