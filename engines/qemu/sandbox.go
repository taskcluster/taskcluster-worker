package qemuengine

import (
	"errors"
	"net/http"
	"strings"
	"sync"

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
	vm            *vm.VirtualMachine
	context       *runtime.TaskContext
	engine        *engine
	proxies       map[string]http.Handler
	metaService   *metaservice.MetaService
	resolve       atomics.Once      // Must wrap access mutation of resultXXX/done
	resultSet     engines.ResultSet // ResultSet for WaitForResult
	resultError   error             // Error for WaitForResult
	resultAbort   error             // Error for Abort
	log           *logrus.Entry     // System log
	mShells       sync.Mutex        // Must be held for newShellError and shells
	shells        []engines.Shell   // List of active shells, guarded by mShells
	newShellError error             // Error, if we're not allowing new shells
	shellsDone    sync.Cond         // Condition to be signaled when shells are done
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
	s.shellsDone.L = &s.mShells

	// Setup meta-data service
	s.metaService = metaservice.New(command, env, c.LogDrain(), s.result, e.Environment)

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
	// Wait for all shells to have finished
	s.mShells.Lock()
	for len(s.shells) > 0 {
		s.shellsDone.Wait()
	}
	// Do now allow new shells
	s.newShellError = engines.ErrSandboxTerminated
	s.mShells.Unlock()

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
		// Kill all shells
		s.abortShells()

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
		s.abortShells()

		// Abort the VM
		s.vm.Kill()
		s.resultError = engines.ErrSandboxAborted
	})

	s.resolve.Wait()
	return s.resultAbort
}

func (s *sandbox) NewShell() (engines.Shell, error) {
	s.mShells.Lock()
	defer s.mShells.Unlock()

	// Check that we still allow creation of new shells. We stop allowing new
	// shells if:
	//  A) We are aborting the sandbox, and is starting to abort all shells
	//  B) Sandbox have returned, and we have waited for all existing shells to
	//     finish.
	// Note: In (B) that we do allow new shells while waiting for existing shells
	//       to finish.
	if s.newShellError != nil {
		return nil, s.newShellError
	}

	// Create new shell
	shell, err := s.metaService.ExecShell()
	if err != nil {
		// Track the shell while running
		s.shells = append(s.shells, shell)
		// Wait for shell to finish and remove it
		go s.waitForShell(shell)
	}

	return shell, err
}

func (s *sandbox) abortShells() {
	// Lock mShell
	s.mShells.Lock()
	defer s.mShells.Unlock()

	// Stop allowing new shells
	s.newShellError = engines.ErrSandboxAborted

	// Call abort() on all shells
	for _, sh := range s.shells {
		sh.Abort()
	}
}

func (s *sandbox) waitForShell(shell engines.Shell) {
	// Wait for shell to finish
	shell.Wait()

	// Lock access to s.shells
	s.mShells.Lock()
	defer s.mShells.Unlock()

	// Remove shell from s.shells
	shells := s.shells[:0]
	for _, sh := range s.shells {
		if sh != shell {
			shells = append(shells, sh)
		}
	}
	s.shells = shells

	// Notify threads waiting if all shells are done
	if len(s.shells) == 0 {
		s.shellsDone.Broadcast()
	}
}
