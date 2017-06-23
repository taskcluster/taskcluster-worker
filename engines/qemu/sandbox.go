package qemuengine

import (
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/pkg/errors"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/metaservice"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/vm"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
)

type sandbox struct {
	engines.SandboxBase
	vm          *vm.VirtualMachine
	context     *runtime.TaskContext
	engine      *engine
	proxies     map[string]http.Handler
	metaService *metaservice.MetaService
	resolve     atomics.Once      // Must wrap access mutation of resultXXX/done
	resultSet   engines.ResultSet // ResultSet for WaitForResult
	resultError error             // Error for WaitForResult
	resultAbort error             // Error for Abort
	monitor     runtime.Monitor   // System log / metrics / error reporting
	sessions    *sessionManager
}

// newSandbox will create a new sandbox and start it.
func newSandbox(
	command []string,
	env map[string]string,
	proxies map[string]http.Handler,
	machine vm.Machine,
	image vm.Image,
	network vm.Network,
	c *runtime.TaskContext,
	e *engine,
	monitor runtime.Monitor,
) (*sandbox, error) {
	instance, err := vm.NewVirtualMachine(
		e.engineConfig.MachineLimits,
		// Merge machine definitions in order of preference:
		//  - task.payload.machine
		//  - machine.json from iamge
		//  - machine from engine config
		//  - default machine (hardcoded into vm.NewVirtualMachine)
		vm.OverwriteMachine(image, machine.WithDefaults(image.Machine()).WithDefaults(e.defaultMachine)),
		network, e.socketFolder.Path(), "", "",
		monitor.WithTag("component", "vm"),
	)
	if err != nil {
		return nil, err
	}

	// Create sandbox
	s := &sandbox{
		vm:      instance,
		context: c,
		engine:  e,
		proxies: proxies,
		monitor: monitor,
	}

	// Setup meta-data service
	s.metaService = metaservice.New(command, env, c.LogDrain(), s.result, e.Environment)

	// Create session manager
	s.sessions = newSessionManager(s.metaService, s.vm)

	// Setup network handler
	s.vm.SetHTTPHandler(http.HandlerFunc(s.handleRequest))

	// Start the VM
	debug("Starting virtual machine")
	s.vm.Start()

	// Resolve when VM is closed
	go s.waitForCrash()

	return s, nil
}

func (s *sandbox) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Sanity checks and identifiation of name/hostname/virtualhost/folder
	var origPath string
	isRawPath := r.URL.RawPath != ""
	if isRawPath {
		origPath = r.URL.RawPath
	} else {
		origPath = r.URL.Path
	}
	if len(origPath) == 0 || origPath[0] != '/' {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	p := strings.SplitN(origPath[1:], "/", 2)
	if len(p) != 2 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	name, path := p[0], "/"+p[1]

	// If name is engine, we pass it to meta-data
	if name == "engine" {
		s.metaService.ServeHTTP(w, r)
		return
	}
	h := s.proxies[name]
	if h == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Rewrite the path
	if isRawPath {
		r.URL.Path, _ = url.PathUnescape(path)
		r.URL.RawPath = path
	} else {
		r.URL.Path = path
		r.URL.RawPath = ""
	}

	h.ServeHTTP(w, r)
}

func (s *sandbox) result(success bool) {
	// Wait for all sessions to be finished and stop issuing new sessions
	debug("ready to resolve success=%v - waiting for shells/displays to finish", success)
	s.sessions.WaitAndTerminate()

	s.resolve.Do(func() {
		s.resultSet = newResultSet(success, s.vm, s.metaService)
		s.resultAbort = engines.ErrSandboxTerminated
	})
}

func (s *sandbox) Kill() error {
	s.resolve.Do(func() {
		s.sessions.KillSessions()
		s.metaService.KillProcess()
		s.resultSet = newResultSet(false, s.vm, s.metaService)
		s.resultAbort = engines.ErrSandboxTerminated
	})
	s.resolve.Wait()
	return s.resultError
}

// waitForCrash will wait for a VM crash and resolve
func (s *sandbox) waitForCrash() {
	// Wait for the VM to finish
	<-s.vm.Done

	s.resolve.Do(func() {
		// Kill all sessions
		s.sessions.AbortSessions()

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
		// Kill all shells
		s.sessions.AbortSessions()

		// Abort the VM
		s.vm.Kill()
		s.resultError = engines.ErrSandboxAborted
	})

	s.resolve.Wait()
	return s.resultAbort
}

func (s *sandbox) NewShell(command []string, tty bool) (engines.Shell, error) {
	return s.sessions.NewShell(command, tty)
}

const qemuDisplayName = "screen"

func (s *sandbox) ListDisplays() ([]engines.Display, error) {
	select {
	case <-s.vm.Done:
		return nil, engines.ErrSandboxTerminated
	default:
		img, _ := s.vm.Screenshot()
		w := 0
		h := 0
		if img != nil {
			w = img.Bounds().Size().X
			h = img.Bounds().Size().Y
		}
		return []engines.Display{
			{
				Name:        qemuDisplayName,
				Description: "Primary screen attached to the virtual machine",
				Width:       w,
				Height:      h,
			},
		}, nil
	}
}

func (s *sandbox) OpenDisplay(name string) (io.ReadWriteCloser, error) {
	if name != qemuDisplayName {
		return nil, engines.ErrNoSuchDisplay
	}
	return s.sessions.OpenDisplay()
}
