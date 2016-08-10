package metaservice

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

// PollTimeout is the maximum amount of time a poll request will be left
// hanging before getting a response (even if the response is none).
const PollTimeout = 30 * time.Second

type asyncCallback func(http.ResponseWriter, *http.Request)
type asyncRecord struct {
	Callback asyncCallback
	Done     chan<- struct{}
}

// MetaService implements the meta-data service that communicates worker process
// running inside the virtual machine. This is how the command to run gets into
// the virtual machine, and how logs and artifacts are copied out.
type MetaService struct {
	m               sync.Mutex
	command         []string
	env             map[string]string
	logDrain        io.Writer
	resultCallback  func(bool)
	environment     *runtime.Environment
	resolved        bool
	result          bool
	mux             *http.ServeMux
	actionOut       chan Action
	pendingRecords  map[string]*asyncRecord
	mPendingRecords sync.Mutex
	haltPolling     chan struct{} // Closed when polling should stop (for tests)
}

// New returns a new MetaService that will tell the virtual machine to
// run command, with environment variables env. It will write the logs to
// logDrain.
//
// The callback resultCallback will be called when the guest reports that the
// command is done.
func New(
	command []string, env map[string]string, logDrain io.Writer,
	resultCallback func(bool), environment *runtime.Environment,
) *MetaService {
	s := &MetaService{
		command:        command,
		env:            env,
		logDrain:       logDrain,
		resultCallback: resultCallback,
		environment:    environment,
		mux:            http.NewServeMux(),
		actionOut:      make(chan Action),
		pendingRecords: make(map[string]*asyncRecord),
		haltPolling:    make(chan struct{}),
	}

	s.mux.HandleFunc("/engine/v1/execute", s.handleExecute)
	s.mux.HandleFunc("/engine/v1/log", s.handleLog)
	s.mux.HandleFunc("/engine/v1/success", s.handleSuccess)
	s.mux.HandleFunc("/engine/v1/failed", s.handleFailed)
	s.mux.HandleFunc("/engine/v1/poll", s.handlePoll)
	s.mux.HandleFunc("/engine/v1/reply", s.handleReply)
	s.mux.HandleFunc("/engine/v1/ping", s.handlePing)
	s.mux.HandleFunc("/", s.handleUnknown)

	return s
}

// ServeHTTP handles request to the meta-data service.
func (s *MetaService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// reply will write status and response to ResponseWriter.
func reply(w http.ResponseWriter, status int, response interface{}) error {
	var data []byte
	if response != nil {
		d, err := json.Marshal(response)
		if err != nil {
			panic(fmt.Sprintf("Failed to marshal %+v to JSON, error: %s", response, err))
		}
		data = d
	}
	w.WriteHeader(status)
	_, err := w.Write(data)
	return err
}

// forceMethod will return true if the request r has the given method.
// Otherwise, it'll return false and send an error message.
func forceMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	reply(w, http.StatusMethodNotAllowed, Error{
		Code:    ErrorCodeMethodNotAllowed,
		Message: fmt.Sprintf("This meta-data API end-point only supports '%s' requests", method),
	})
	return false
}

// handleExecute handles GET /engine/v1/execute
func (s *MetaService) handleExecute(w http.ResponseWriter, r *http.Request) {
	if !forceMethod(w, r, http.MethodGet) {
		return
	}

	debug("GET /engine/v1/execute")
	reply(w, http.StatusOK, Execute{
		Command: s.command,
		Env:     s.env,
	})
}

// handleLog handles with POST /engine/v1/log
func (s *MetaService) handleLog(w http.ResponseWriter, r *http.Request) {
	if !forceMethod(w, r, http.MethodPost) {
		return
	}

	debug("POST /engine/v1/log")
	_, err := io.Copy(s.logDrain, r.Body)
	if err != nil {
		reply(w, http.StatusInternalServerError, Error{
			Code:    ErrorCodeInternalError,
			Message: "Error while writing log, please ensure the task is completed",
		})
		return
	}

	reply(w, http.StatusOK, nil)
}

// handleSuccess handles PUT /engine/v1/success
func (s *MetaService) handleSuccess(w http.ResponseWriter, r *http.Request) {
	if !forceMethod(w, r, http.MethodPut) {
		return
	}

	// Only resolve once
	s.m.Lock()
	resolved := s.resolved
	if !s.resolved {
		s.result = true
	}
	s.resolved = true
	s.m.Unlock()

	debug("PUT /engine/v1/success")
	if s.result {
		if !resolved && s.resultCallback != nil {
			s.resultCallback(true)
		}
		reply(w, http.StatusOK, nil)
	} else {
		reply(w, http.StatusConflict, Error{
			Code:    ErrorCodeResourceConflict,
			Message: "The task have already been resolved failed, you must have a bug.",
		})
	}
}

// handleFailed handles PUT /engine/v1/failed
func (s *MetaService) handleFailed(w http.ResponseWriter, r *http.Request) {
	if !forceMethod(w, r, http.MethodPut) {
		return
	}

	// Only resolve once
	s.m.Lock()
	resolved := s.resolved
	if !s.resolved {
		s.result = false
	}
	s.resolved = true
	s.m.Unlock()

	debug("PUT /engine/v1/failed")
	if !s.result {
		if !resolved && s.resultCallback != nil {
			s.resultCallback(false)
		}
		reply(w, http.StatusOK, nil)
	} else {
		reply(w, http.StatusConflict, Error{
			Code:    ErrorCodeResourceConflict,
			Message: "The task have already been resolved success, you must have a bug.",
		})
	}
}

// handlePing handles ping requests
func (s *MetaService) handlePing(w http.ResponseWriter, r *http.Request) {
	if !forceMethod(w, r, http.MethodGet) {
		return
	}

	debug("GET /engine/v1/ping")
	reply(w, http.StatusOK, nil)
}

// handleUnknown handles unhandled requests
func (s *MetaService) handleUnknown(w http.ResponseWriter, r *http.Request) {
	debug("Unhandled request: %+v", r)
	reply(w, http.StatusNotFound, Error{
		Code:    ErrorCodeNoSuchEndPoint,
		Message: "Unknown meta-data API end-point",
	})
}

// StopPollers will stop all polling requests. This is only used for testing,
// where it is important that clients stop polling or we can't Close the server.
func (s *MetaService) StopPollers() {
	select {
	case <-s.haltPolling:
	default:
		close(s.haltPolling)
	}
}

// handlePoll handles GET /engine/v1/poll
func (s *MetaService) handlePoll(w http.ResponseWriter, r *http.Request) {
	if !forceMethod(w, r, http.MethodGet) {
		return
	}

	debug("GET /engine/v1/poll")
	select {
	case <-s.haltPolling:
		reply(w, http.StatusOK, Action{
			ID:   slugid.Nice(),
			Type: "none",
		})
	case <-time.After(PollTimeout):
		reply(w, http.StatusOK, Action{
			ID:   slugid.Nice(),
			Type: "none",
		})
	case action := <-s.actionOut:
		debug(" -> Sending action with id=%s", action.ID)
		if reply(w, http.StatusOK, action) != nil {
			debug("Failed to send action id=%s", action.ID)

			// Take the asyncRecord record out of the dictionary
			s.mPendingRecords.Lock()
			rec := s.pendingRecords[action.ID]
			delete(s.pendingRecords, action.ID)
			s.mPendingRecords.Unlock()

			// If nil, then the request is already being handled, and there is no need
			// to abort (presumably the action we received on the other side)
			if rec != nil {
				close(rec.Done)
			}
		}
	}
}

// asyncRequest will return action to the current (or next) request to
// GET /engine/v1/poll, then it'll wait for a POST request to /engine/v1/reply
// with matching id in querystring and forward this request to cb.
func (s *MetaService) asyncRequest(action Action, cb asyncCallback) {
	// Ensure the action has a unique id
	action.ID = slugid.Nice()

	// Create channel to track when the callback has been called
	isDone := make(chan struct{})
	rec := asyncRecord{
		Callback: cb,
		Done:     isDone,
	}

	// Insert asyncRecord is pending set
	s.mPendingRecords.Lock()
	s.pendingRecords[action.ID] = &rec
	s.mPendingRecords.Unlock()

	// Send action
	select {
	case <-time.After(30 * time.Second):
		// If sending times out we delete the record
		s.mPendingRecords.Lock()
		delete(s.pendingRecords, action.ID)
		s.mPendingRecords.Unlock()
		return
	case s.actionOut <- action:
	}

	// Wait for async callback to have been called
	select {
	case <-time.After(30 * time.Second):
		// if we timeout, we take the async record
		s.mPendingRecords.Lock()
		rec := s.pendingRecords[action.ID]
		delete(s.pendingRecords, action.ID)
		s.mPendingRecords.Unlock()

		// if there was a record, we've removed it and we're done...
		if rec != nil {
			return
		}
		// if there was no record, we have to wait for isDone as a request must
		// be in the process executing the callback
		<-isDone
	case <-isDone:
	}
}

// handleReply handles
func (s *MetaService) handleReply(w http.ResponseWriter, r *http.Request) {
	// Get action id
	id := r.URL.Query().Get("id")
	debug("%s /engine/v1/reply?id=%s", r.Method, id)

	// if we timeout, we take the async record
	s.mPendingRecords.Lock()
	rec := s.pendingRecords[id]
	delete(s.pendingRecords, id)
	s.mPendingRecords.Unlock()

	// If there is no record of this action, we just return 400
	if rec == nil {
		reply(w, http.StatusBadRequest, Error{
			Code: ErrorCodeUnknownActionID,
			Message: fmt.Sprintf(
				"Action id: '%s' is not known, perhaps it has timed out.", id,
			),
		})
		return
	}

	// Call the callback from the record and mark it done.
	defer close(rec.Done)
	rec.Callback(w, r)
}

func (s *MetaService) getArtifactWithoutRetry(path string) (
	ioext.ReadSeekCloser, error,
) {
	// Create result values to be set in the callback
	var File ioext.ReadSeekCloser
	var Err error
	Err = engines.ErrNonFatalInternalError

	s.asyncRequest(Action{
		Type: "get-artifact",
		Path: path,
	}, func(w http.ResponseWriter, r *http.Request) {
		if !forceMethod(w, r, http.MethodPost) {
			return
		}

		// Return an ErrResourceNotFound error, if file could not be found
		if r.Header.Get("X-Taskcluster-Worker-Error") == "file-not-found" {
			reply(w, http.StatusOK, nil)
			Err = engines.ErrResourceNotFound
			return
		}

		// Create a temporary file
		f, err := s.environment.TemporaryStorage.NewFile()
		if err != nil {
			reply(w, http.StatusInternalServerError, Error{
				Code:    ErrorCodeInternalError,
				Message: "Unable to write file to disk",
			})
			return
		}

		// Copy body to temporary file
		_, err = io.Copy(f, r.Body)
		if err != nil {
			f.Close() // Release temporary file
			reply(w, http.StatusInternalServerError, Error{
				Code:    ErrorCodeInternalError,
				Message: "Error copying request body to disk",
			})
			return
		}

		// Reply 200 OK (as client didn't do anything wrong)
		reply(w, http.StatusOK, nil)

		// Seek to start of temporary file
		_, err = f.Seek(0, 0)
		if err != nil {
			f.Close() // Release temporary file
			return
		}

		File = f
		Err = nil
	})

	return File, Err
}

// GetArtifact will tell polling guest-tools to send a given artifact.
func (s *MetaService) GetArtifact(path string) (ioext.ReadSeekCloser, error) {
	retries := 3
	for {
		f, err := s.getArtifactWithoutRetry(path)
		retries--
		if err == engines.ErrNonFatalInternalError && retries > 0 {
			continue
		}
		return f, err
	}
}

func (s *MetaService) listFolderWithoutRetries(path string) ([]string, error) {
	var Result []string
	var Err error
	Err = engines.ErrNonFatalInternalError

	s.asyncRequest(Action{
		Type: "list-folder",
		Path: path,
	}, func(w http.ResponseWriter, r *http.Request) {
		if !forceMethod(w, r, http.MethodPost) {
			return
		}

		// Check content-type
		if r.Header.Get("Content-Type") != "application/json" {
			reply(w, http.StatusBadRequest, Error{
				Code:    ErrorCodeInvalidPayload,
				Message: "Content-Type must be application/json",
			})
			return
		}

		// Read first 10 MiB of body (limited for safety)
		body, err := ioutil.ReadAll(io.LimitReader(r.Body, 10*1024*1024))
		if err != nil {
			reply(w, http.StatusInternalServerError, Error{
				Code:    ErrorCodeInternalError,
				Message: "Failed to read request body",
			})
			return
		}
		// Parse JSON payload
		p := Files{}
		if json.Unmarshal(body, &p) != nil {
			reply(w, http.StatusBadRequest, Error{
				Code:    ErrorCodeInvalidPayload,
				Message: "Invalid JSON payload",
			})
			return
		}

		// If body parse we return OK
		reply(w, http.StatusOK, nil)
		if p.NotFound {
			Err = engines.ErrResourceNotFound
		} else {
			Err = nil
			Result = p.Files
		}
	})

	return Result, Err
}

// ListFolder the contents of a folder (recursively)
func (s *MetaService) ListFolder(path string) ([]string, error) {
	retries := 3
	for {
		files, err := s.listFolderWithoutRetries(path)
		retries--
		if err == engines.ErrNonFatalInternalError && retries > 0 {
			continue
		}
		return files, err
	}
}

var upgrader = websocket.Upgrader{
	HandshakeTimeout: ShellHandshakeTimeout,
	ReadBufferSize:   ShellMaxMessageSize,
	WriteBufferSize:  ShellMaxMessageSize,
}

// ExecShell will send an action to guest-tools to execute a shell, then wait
// for guest-tools to callback establish a websocket and connect to an
// implementation of engines.Shell
func (s *MetaService) ExecShell() (engines.Shell, error) {
	var Shell engines.Shell
	var Err error

	s.asyncRequest(Action{
		Type: "exec-shell",
	}, func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			debug("Failed to upgrade request to websocket, error: %s", err)
			Err = engines.ErrNonFatalInternalError
			return
		}

		Shell = newShell(ws)
	})

	return Shell, Err
}
