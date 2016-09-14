package interactive

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/websocket"
	"github.com/taskcluster/taskcluster-worker/engines"
)

type displayServer struct {
	m        sync.Mutex
	sandbox  engines.Sandbox
	log      *logrus.Entry
	done     chan struct{}
	handlers []*DisplayHandler
}

func newDisplayServer(sandbox engines.Sandbox, log *logrus.Entry) *displayServer {
	return &displayServer{
		log:     log,
		sandbox: sandbox,
		done:    make(chan struct{}),
	}
}

func (s *displayServer) Abort() {
	s.m.Lock()
	defer s.m.Unlock()

	select {
	case <-s.done:
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
	HandshakeTimeout: 30 * time.Second,
	ReadBufferSize:   displayBufferSize,
	WriteBufferSize:  displayBufferSize,
}

func (s *displayServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	select {
	case <-s.done:
		reply(w, http.StatusGone, errorMessage{
			Code:    "ExecutionTerminated",
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
		reply(w, http.StatusBadRequest, errorMessage{
			Code:    "InvalidParameters",
			Message: "Querystring parameter 'display' must given!",
		})
	}
	display, err := s.sandbox.OpenDisplay(displayName)
	if err == engines.ErrNoSuchDisplay {
		reply(w, http.StatusNotFound, errorMessage{
			Code:    "DisplayNotFound",
			Message: fmt.Sprintf("Display: '%s' couldn't be found", displayName),
		})
		return
	}
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
	h := NewDisplayHandler(ws, display, s.log.WithField("display", displayName))
	s.handlers = append(s.handlers, h)
}

func (s *displayServer) listDisplays(w http.ResponseWriter, r *http.Request) {
	displays, err := s.sandbox.ListDisplays()
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
	result := make([]displayEntry, len(displays))
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
		data, _ = json.Marshal(payload)
	}
	if len(data) > 0 {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(status)
	w.Write(data)
}

type errorMessage struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type displayEntry struct {
	Display     string `json:"display"`
	Description string `json:"description"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
}

var errorMessageDisplayNotSupported = errorMessage{
	Code:    "DisplayNotFound",
	Message: "Task execution environment doesn't support display interaction",
}

var errorMessageExecutionHalted = errorMessage{
	Code:    "ExecutionTerminated",
	Message: "Task execution has halted, displays are not available anymore.",
}

var errorMessageInternalError = errorMessage{
	Code:    "InternalError",
	Message: "Worker encountered an internal error",
}
