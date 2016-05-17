package metaservice

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
)

// MetaService implements the meta-data service that communicates worker process
// running inside the virtual machine. This is how the command to run gets into
// the virtual machine, and how logs and artifacts are copied out.
type MetaService struct {
	m              sync.Mutex
	command        []string
	env            map[string]string
	logDrain       io.Writer
	resultCallback func(bool)
	resolved       bool
	result         bool
	mux            *http.ServeMux
}

// New returns a new MetaService that will tell the virtual machine to
// run command, with environment variables env. It will write the logs to
// logDrain.
//
// The callback resultCallback will be called when the guest reports that the
// command is done.
func New(command []string, env map[string]string, logDrain io.Writer, resultCallback func(bool)) *MetaService {
	s := &MetaService{
		command:        command,
		env:            env,
		logDrain:       logDrain,
		resultCallback: resultCallback,
		mux:            http.NewServeMux(),
	}

	s.mux.HandleFunc("/engine/v1/execute", s.handleExecute)
	s.mux.HandleFunc("/engine/v1/log", s.handleLog)
	s.mux.HandleFunc("/engine/v1/success", s.handleSuccess)
	s.mux.HandleFunc("/engine/v1/failed", s.handleFailed)
	s.mux.HandleFunc("/", s.handleUnknown)

	return s
}

// ServeHTTP handles request to the meta-data service.
func (s *MetaService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// reply will write status and response to ResponseWriter.
func reply(w http.ResponseWriter, status int, response interface{}) {
	var data []byte
	if response != nil {
		d, err := json.Marshal(response)
		if err != nil {
			panic(fmt.Sprintf("Failed to marshal %+v to JSON, error: %s", response, err))
		}
		data = d
	}
	w.WriteHeader(status)
	w.Write(data)
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
	reply(w, http.StatusNotFound, Error{
		Code:    ErrorCodeNoSuchEndPoint,
		Message: "Unknown meta-data API end-point",
	})
}
