package metaservice

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

type artifactResult struct {
	File ioext.ReadSeekCloser
	Err  error
	Done chan<- struct{}
}

type listFolderResult struct {
	Files []string
	Err   error
	Done  chan<- struct{}
}

// MetaService implements the meta-data service that communicates worker process
// running inside the virtual machine. This is how the command to run gets into
// the virtual machine, and how logs and artifacts are copied out.
type MetaService struct {
	m                   sync.Mutex
	command             []string
	env                 map[string]string
	logDrain            io.Writer
	resultCallback      func(bool)
	environment         *runtime.Environment
	resolved            bool
	result              bool
	mux                 *http.ServeMux
	actionOut           chan Action
	pendingArtifacts    map[string]*artifactResult
	mPendingArtifacts   sync.Mutex
	pendingListFolders  map[string]*listFolderResult
	mPendingListFolders sync.Mutex
}

// New returns a new MetaService that will tell the virtual machine to
// run command, with environment variables env. It will write the logs to
// logDrain.
//
// The callback resultCallback will be called when the guest reports that the
// command is done.
func New(command []string, env map[string]string, logDrain io.Writer, resultCallback func(bool), environment *runtime.Environment) *MetaService {
	s := &MetaService{
		command:          command,
		env:              env,
		logDrain:         logDrain,
		resultCallback:   resultCallback,
		environment:      environment,
		mux:              http.NewServeMux(),
		actionOut:        make(chan Action),
		pendingArtifacts: make(map[string]*artifactResult),
	}

	s.mux.HandleFunc("/engine/v1/execute", s.handleExecute)
	s.mux.HandleFunc("/engine/v1/log", s.handleLog)
	s.mux.HandleFunc("/engine/v1/success", s.handleSuccess)
	s.mux.HandleFunc("/engine/v1/failed", s.handleFailed)
	s.mux.HandleFunc("/engine/v1/poll", s.handlePoll)
	s.mux.HandleFunc("/engine/v1/artifact", s.handleArtifact)
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

// handleUnknown handles unhandled requests
func (s *MetaService) handleUnknown(w http.ResponseWriter, r *http.Request) {
	debug("Unhandled request: %+v", r)
	reply(w, http.StatusNotFound, Error{
		Code:    ErrorCodeNoSuchEndPoint,
		Message: "Unknown meta-data API end-point",
	})
}

// handlePoll handles GET /engine/v1/poll
func (s *MetaService) handlePoll(w http.ResponseWriter, r *http.Request) {
	if !forceMethod(w, r, http.MethodGet) {
		return
	}

	debug("GET /engine/v1/poll")
	select {
	case <-time.After(30 * time.Second):
		reply(w, http.StatusOK, Action{
			ID:   slugid.V4(),
			Type: "none",
		})
	case action := <-s.actionOut:
		err := reply(w, http.StatusOK, action)
		if err != nil {
			// Take the artifactResult record out of the dictionary
			s.mPendingArtifacts.Lock()
			result := s.pendingArtifacts[action.ID]
			delete(s.pendingArtifacts, action.ID)
			s.mPendingArtifacts.Unlock()

			// If nil, then the request is already being handled, and there is no need
			// to abort (presumably the action we received on the other side)
			if result != nil {
				result.Err = engines.ErrNonFatalInternalError
				close(result.Done)
			}
		}
	}
}

// handleArtifact handles request uploading an artifact in response to a
// get-artifact action.
func (s *MetaService) handleArtifact(w http.ResponseWriter, r *http.Request) {
	if !forceMethod(w, r, http.MethodPost) {
		return
	}

	// Get action id that this artifact is being posted for
	id := r.URL.Query().Get("id")

	// Take the artifactResult record out of the dictionary
	s.mPendingArtifacts.Lock()
	result := s.pendingArtifacts[id]
	delete(s.pendingArtifacts, id)
	s.mPendingArtifacts.Unlock()

	// If there was no result record, we're done
	if result == nil {
		reply(w, http.StatusBadRequest, Error{
			Code: ErrorCodeUnknownActionID,
			Message: fmt.Sprintf(
				"Action id: '%s' is not known, perhaps it has timed out.", id,
			),
		})
		return
	}

	// If header indicates a 404, then we return an ErrResourceNotFound error
	if r.Header.Get("X-Taskcluster-Worker-Error") == "file-not-found" {
		reply(w, http.StatusOK, nil)
		result.Err = engines.ErrResourceNotFound
		close(result.Done)
		return
	}

	// Create a temporary file
	f, err := s.environment.TemporaryStorage.NewFile()
	if err != nil {
		reply(w, http.StatusInternalServerError, Error{
			Code:    ErrorCodeInternalError,
			Message: "Unable to write file to disk",
		})
		result.Err = engines.ErrNonFatalInternalError
		close(result.Done)
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
		result.Err = engines.ErrNonFatalInternalError
		close(result.Done)
		return
	}

	// Reply 200 OK (as client didn't do anything wrong)
	reply(w, http.StatusOK, nil)

	// Seek to start of temporary file
	_, err = f.Seek(0, 0)
	if err != nil {
		f.Close() // Release temporary file
		result.Err = engines.ErrNonFatalInternalError
		close(result.Done)
		return
	}

	// Return temporary file as result
	result.File = f
	close(result.Done)
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

func (s *MetaService) getArtifactWithoutRetry(path string) (ioext.ReadSeekCloser, error) {
	// Create get-artifact action (to be sent)
	action := Action{
		ID:   slugid.V4(),
		Type: "get-artifact",
		Path: path,
	}

	// Create isDone channel and result record
	isDone := make(chan struct{})
	result := artifactResult{
		Done: isDone,
	}

	// Insert result record
	s.mPendingArtifacts.Lock()
	s.pendingArtifacts[action.ID] = &result
	s.mPendingArtifacts.Unlock()

	// Send action to guest-tools
	select {
	case <-time.After(30 * time.Second):
		s.mPendingArtifacts.Lock()
		delete(s.pendingArtifacts, action.ID)
		s.mPendingArtifacts.Unlock()
		return nil, engines.ErrNonFatalInternalError
	case s.actionOut <- action:
	}

	// Wait for result from guest-tools
	select {
	case <-time.After(30 * time.Second):
		// Take the result from the pendingArtifacts
		s.mPendingArtifacts.Lock()
		res := s.pendingArtifacts[action.ID]
		delete(s.pendingArtifacts, action.ID)
		s.mPendingArtifacts.Unlock()

		// if it was present in pendingArtifacts, then we're done
		if res != nil {
			return nil, engines.ErrNonFatalInternalError
		}
		// If pendingArtifacts[action.ID] was nil, then a request has arrived
		// we just wait for the isDone to be resolve with an error or something.
		<-isDone
	case <-isDone:
	}

	return result.File, result.Err
}

/*
func (s *MetaService) listFolder(path string) ([]string, error) {
	// Create list-folder action (to be sent)
	action := Action{
		ID:   slugid.V4(),
		Type: "list-folder",
		Path: path,
	}

}
*/
