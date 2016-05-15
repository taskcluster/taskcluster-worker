package qemuengine

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/image"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/network"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
)

type sandbox struct {
	engines.SandboxBase
	vm          *virtualMachine
	command     []string
	context     *runtime.TaskContext
	engine      *engine
	proxies     map[string]http.Handler
	metaMux     *http.ServeMux
	resolve     atomics.Once      // Must wrap access mutation of resultXXX/done
	resultSet   engines.ResultSet // ResultSet for WaitForResult
	resultError error             // Error for WaitForResult
	resultAbort error             // Error for Abort
}

// newSandbox will create a new sandbox and start it.
func newSandbox(
	command []string,
	proxies map[string]http.Handler,
	image *image.Instance,
	network *network.Network,
	c *runtime.TaskContext,
	e *engine,
) *sandbox {
	// Create sandbox
	s := &sandbox{
		vm:      newVirtualMachine(image, network, e.engineConfig.SocketFolder),
		command: command,
		context: c,
		engine:  e,
		proxies: proxies,
		metaMux: http.NewServeMux(),
	}

	// Setup meta-data muxer
	s.metaMux.HandleFunc("/engine/v1/command", s.handleCommand)
	s.metaMux.HandleFunc("/engine/v1/log", s.handleLog)
	s.metaMux.HandleFunc("/engine/v1/success", s.handleSuccess)
	s.metaMux.HandleFunc("/engine/v1/failed", s.handleFailed)

	// Setup network handler
	s.vm.SetHTTPHandler(http.HandlerFunc(s.handleRequest))

	// Start the VM
	s.vm.Start()

	// Resolve when VM is closed
	go s.waitForCrash()

	return s
}

func (s *sandbox) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Sanity checks and identifiation of name/hostname/virtualhost/folder
	if r.URL.Path[0] != '/' {
		w.WriteHeader(404)
		return
	}
	p := strings.SplitN(r.URL.Path[1:], "/", 2)
	if len(p) != 2 {
		w.WriteHeader(404)
		return
	}
	name, path := p[0], "/"+p[1]

	// If name is engine, we pass it to meta-data
	if name == "engine" {
		s.metaMux.ServeHTTP(w, r)
	}
	h := s.proxies[name]
	if h == nil {
		w.WriteHeader(404)
		return
	}
	r.URL.Path = path
	r.URL.RawPath = "" // TODO: implement this if we ever need it
	h.ServeHTTP(w, r)
}

// handleCommand deals with GET /engine/v1/command
func (s *sandbox) handleCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// TODO: make all this API end-points operate on JSON
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(strings.Join(s.command, " ")))
}

// handleCommand deals with POST /engine/v1/log
func (s *sandbox) handleLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	_, err := io.Copy(s.context.LogDrain(), r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error writing log, not entirely sure what the error was."))
		return
	}

	w.WriteHeader(http.StatusOK)
}

// handleCommand deals with PUT /engine/v1/success
func (s *sandbox) handleSuccess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if s.result(true) {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusConflict)
	}
}

// handleCommand deals with PUT /engine/v1/failed
func (s *sandbox) handleFailed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if s.result(false) {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusConflict)
	}
}

func (s *sandbox) result(success bool) bool {
	return s.resolve.Do(func() {
		s.resultSet = newResultSet(success, s.vm)
		s.resultAbort = engines.ErrSandboxTerminated
	})
}

// waitForCrash will wait for a VM crash and resolve
func (s *sandbox) waitForCrash() {
	// Wait for the VM to finish
	<-s.vm.Done

	s.resolve.Do(func() {
		// TODO: Read s.vm.Error and handle the error
		s.resultError = errors.New("QEMU crashed unexpected")
		s.resultAbort = engines.ErrSandboxTerminated
	})
}

func (s *sandbox) WaitForResult() (engines.ResultSet, error) {
	s.resolve.Wait()
	return s.resultSet, s.resultError
}

func (s *sandbox) Abort() error {
	s.resolve.Do(func() {
		// Abort the VM
		s.vm.Kill()
		s.resultError = engines.ErrSandboxAborted
	})

	s.resolve.Wait()
	return s.resultAbort
}
