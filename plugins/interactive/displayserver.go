package interactive

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins/interactive/displayconsts"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

// A DisplayProvider is an object that supplies displays. This is a subset of
// the Sandbox interface.
type DisplayProvider interface {
	// See engines.Sandbox for documentation for methods.
	ListDisplays() ([]engines.Display, error)
	OpenDisplay(name string) (io.ReadWriteCloser, error)
}

// A DisplayServer exposes a DisplayProvider over a websocket, tracks
// connections and ensures they are all cleaned up.
type DisplayServer struct {
	m        sync.Mutex
	provider DisplayProvider
	monitor  runtime.Monitor
	done     chan struct{}
	handlers []*DisplayHandler
}

// NewDisplayServer creates a DisplayServer for exposing the given provider
// over a websocket.
func NewDisplayServer(provider DisplayProvider, monitor runtime.Monitor) *DisplayServer {
	return &DisplayServer{
		monitor:  monitor,
		provider: provider,
		done:     make(chan struct{}),
	}
}

// Abort stops new display connections from opneing and aborts all existing
// connections, cleaning up all resources held.
func (s *DisplayServer) Abort() {
	s.m.Lock()
	defer s.m.Unlock()

	// Ensure the done channel is closed
	select {
	case <-s.done: // can't close twice
	default:
		close(s.done)
	}

	// Abort all existing handlers
	for _, h := range s.handlers {
		h.Abort()
	}
	s.handlers = nil
}

var displayUpgrader = websocket.Upgrader{
	HandshakeTimeout: displayconsts.DisplayHandshakeTimeout,
	ReadBufferSize:   displayconsts.DisplayBufferSize,
	WriteBufferSize:  displayconsts.DisplayBufferSize,
	CheckOrigin:      func(_ *http.Request) bool { return true },
	Subprotocols:     []string{"binary"},
}

func (s *DisplayServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	select {
	case <-s.done:
		reply(w, http.StatusGone, displayconsts.ErrorMessage{
			Code:    displayconsts.ErrorCodeExecutionTerminated,
			Message: "Task execution has halted, displays are not available anymore.",
		})
		return
	default:
	}

	// If not a websocket upgrade we seek to list the displays
	if !websocket.IsWebSocketUpgrade(r) {
		s.listDisplays(w, r)
		return
	}

	displayName := r.URL.Query().Get("display")
	if displayName == "" {
		reply(w, http.StatusBadRequest, displayconsts.ErrorMessage{
			Code:    displayconsts.ErrorCodeInvalidParameters,
			Message: "Querystring parameter 'display' must be given!",
		})
	}
	display, err := s.provider.OpenDisplay(displayName)
	switch err {
	case engines.ErrNoSuchDisplay:
		reply(w, http.StatusNotFound, displayconsts.ErrorMessage{
			Code:    displayconsts.ErrorCodeDisplayNotFound,
			Message: fmt.Sprintf("Display: '%s' couldn't be found", displayName),
		})
		return
	case engines.ErrSandboxTerminated, engines.ErrSandboxAborted:
		reply(w, http.StatusGone, errorMessageExecutionHalted)
		return
	case engines.ErrFeatureNotSupported:
		reply(w, http.StatusBadRequest, errorMessageDisplayNotSupported)
		return
	case nil:
	default:
		//TODO: Send error to sentry
		reply(w, http.StatusInternalServerError, errorMessageInternalError)
		return
	}

	// Upgrade the connection
	ws, err := displayUpgrader.Upgrade(w, r, nil)
	if err != nil {
		display.Close()
		return
	}

	// Lock and ensure that we haven't aborted
	s.m.Lock()
	defer s.m.Unlock()

	select {
	case <-s.done:
		ws.Close()
		display.Close()
		return
	default:
	}

	// Create new handler and add it to the list
	h := NewDisplayHandler(ws, display, s.monitor.WithTag("display", displayName))
	s.handlers = append(s.handlers, h)
}

func (s *DisplayServer) listDisplays(w http.ResponseWriter, r *http.Request) {
	displays, err := s.provider.ListDisplays()
	if err == engines.ErrSandboxTerminated || err == engines.ErrSandboxAborted {
		reply(w, http.StatusGone, errorMessageExecutionHalted)
		return
	}
	if err == engines.ErrFeatureNotSupported {
		reply(w, http.StatusBadRequest, errorMessageDisplayNotSupported)
		return
	}
	if err != nil {
		//TODO: Send error to sentry
		reply(w, http.StatusInternalServerError, errorMessageInternalError)
		return
	}

	// Convert to JSON...
	result := make([]displayconsts.DisplayEntry, len(displays))
	for i, d := range displays {
		result[i].Description = d.Description
		result[i].Display = d.Name
		result[i].Width = d.Width
		result[i].Height = d.Height
	}

	reply(w, http.StatusOK, result)
}

func reply(w http.ResponseWriter, status int, payload interface{}) {
	var data []byte
	if payload != nil {
		var err error
		data, err = json.Marshal(payload)
		if err != nil {
			panic(fmt.Sprintf("Failed to marshal JSON reply, error: %s", err))
		}
	}
	setCORS(w)
	if len(data) > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	}
	w.WriteHeader(status)
	w.Write(data)
}

var errorMessageDisplayNotSupported = displayconsts.ErrorMessage{
	Code:    displayconsts.ErrorCodeDisplayNotFound,
	Message: "Task execution environment doesn't support display interaction",
}

var errorMessageExecutionHalted = displayconsts.ErrorMessage{
	Code:    displayconsts.ErrorCodeExecutionTerminated,
	Message: "Task execution has halted, displays are not available anymore.",
}

var errorMessageInternalError = displayconsts.ErrorMessage{
	Code:    displayconsts.ErrorCodeInternalError,
	Message: "Worker encountered an internal error",
}
