package qemuengine

import (
	"io"
	"net"
	"sync"

	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/metaservice"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/vm"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

// sessionManager is responsible for creating, tracking and aborting shells and
// displays. This ensures that shells and displays can be created until:
//   a) Execution is aborted, or
//   b) The task is done and the all active shells/displays will end.
//
// The sessionManager is basically responsible for isolating the shell and
// display tracking logic.
type sessionManager struct {
	meta     *metaservice.MetaService
	vm       *vm.VirtualMachine
	m        sync.Mutex           // Must be held for newShellError, shells, displays
	empty    sync.Cond            // Condition signaled when shells/displays are empty
	newError error                // Error, if we don't allow new shells/displays
	shells   []engines.Shell      // Active shells
	displays []io.ReadWriteCloser // Active displays
}

func newSessionManager(meta *metaservice.MetaService, vm *vm.VirtualMachine) *sessionManager {
	s := &sessionManager{
		meta: meta,
		vm:   vm,
	}
	s.empty.L = &s.m
	return s
}

func (s *sessionManager) AbortSessions() {
	// Lock mShell
	s.m.Lock()
	defer s.m.Unlock()

	// Stop allowing new shells
	s.newError = engines.ErrSandboxAborted

	// Call abort() on all shells
	for _, sh := range s.shells {
		sh.Abort()
	}

	// Call close on all display connections
	for _, c := range s.displays {
		c.Close()
	}
}

func (s *sessionManager) WaitAndTerminate() {
	// Wait for all shells to have finished
	s.m.Lock()
	for len(s.shells) > 0 && len(s.displays) > 0 {
		s.empty.Wait()
	}
	// Do now allow new shells
	s.newError = engines.ErrSandboxTerminated
	s.m.Unlock()
}

func (s *sessionManager) NewShell() (engines.Shell, error) {
	s.m.Lock()
	defer s.m.Unlock()

	// Check that we still allow creation of new shells. We stop allowing new
	// shells if:
	//  A) We are aborting the sandbox, and is starting to abort all shells
	//  B) Sandbox have returned, and we have waited for all existing shells to
	//     finish.
	// Note: In (B) that we do allow new shells while waiting for existing shells
	//       to finish.
	if s.newError != nil {
		return nil, s.newError
	}

	// Create new shell
	shell, err := s.meta.ExecShell()
	if err != nil {
		// Track the shell while running
		s.shells = append(s.shells, shell)
		// Wait for shell to finish and remove it
		go s.waitForShell(shell)
	}

	return shell, err
}

func (s *sessionManager) waitForShell(shell engines.Shell) {
	// Wait for shell to finish
	shell.Wait()

	// Lock access to s.shells
	s.m.Lock()
	defer s.m.Unlock()

	// Remove shell from s.shells
	shells := s.shells[:0]
	for _, sh := range s.shells {
		if sh != shell {
			shells = append(shells, sh)
		}
	}
	s.shells = shells

	// Notify threads waiting if all shells are done
	if len(s.shells) == 0 && len(s.displays) == 0 {
		s.empty.Broadcast()
	}
}

func (s *sessionManager) OpenDisplay() (io.ReadWriteCloser, error) {
	// Check if we're still allowing creation of interactive sessions
	s.m.Lock()
	if s.newError != nil {
		return nil, s.newError
	}
	s.m.Unlock()

	// Get socket
	socket := s.vm.VNCSocket()
	if socket == "" {
		// If zero value, then the sandbox is aborted or terminated.
		return nil, engines.ErrSandboxTerminated
	}

	// Dial-up the socket
	conn, err := net.Dial("unix", socket)
	if err != nil {
		// TODO: Check if vm is still running, if so report an error
		return nil, engines.ErrSandboxTerminated
	}

	// Lock we so we can insert in the list of displays
	s.m.Lock()
	defer s.m.Unlock()

	// Create a WatchPipe around conn, so that we can remove it from displays
	// when it is closed
	var display io.ReadWriteCloser
	display = ioext.WatchPipe(conn, func(_ error) {
		// Lock so we can move display from displays
		s.m.Lock()
		defer s.m.Unlock()

		// Remove shell from s.shells
		displays := s.displays[:0]
		for _, d := range s.displays {
			if d != display {
				displays = append(displays, d)
			}
		}
		s.displays = displays

		// Signal threads if displays is empty
		if len(s.shells) == 0 && len(s.displays) == 0 {
			s.empty.Broadcast()
		}
	})
	s.displays = append(s.displays, display)

	return display, nil
}
