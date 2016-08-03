package qemuengine

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/image"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/metaservice"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/network"
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
	log         *logrus.Entry     // System log
	sessions    *sessionManager
}

// newSandbox will create a new sandbox and start it.
func newSandbox(
	command []string,
	env map[string]string,
	proxies map[string]http.Handler,
	image *image.Instance,
	network *network.Network,
	c *runtime.TaskContext,
	e *engine,
) *sandbox {
	log := e.Log.WithField("taskId", c.TaskID).WithField("runId", c.RunID)

	// Create sandbox
	s := &sandbox{
		vm: vm.NewVirtualMachine(
			image, network, e.engineConfig.SocketFolder, "", "",
			log.WithField("component", "vm"),
		),
		context: c,
		engine:  e,
		proxies: proxies,
		log:     log,
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

	return s
}

func (s *sandbox) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Sanity checks and identifiation of name/hostname/virtualhost/folder
	if r.URL.Path[0] != '/' {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	p := strings.SplitN(r.URL.Path[1:], "/", 2)
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
	r.URL.Path = path
	r.URL.RawPath = "" // TODO: implement this if we ever need it
	h.ServeHTTP(w, r)
}

func (s *sandbox) result(success bool) {
	// Wait for all sessions to be finished and stop issuing new sessions
	s.sessions.WaitAndTerminate()

	s.resolve.Do(func() {
		s.resultSet = newResultSet(success, s.vm, s.metaService)
		s.resultAbort = engines.ErrSandboxTerminated
	})
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

func (s *sandbox) NewShell() (engines.Shell, error) {
	return s.sessions.NewShell()
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
