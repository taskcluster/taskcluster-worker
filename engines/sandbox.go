package engines

import "io"

// The Shell interface opens an interactive sh or bash shell inside the Sandbox.
type Shell interface {
	StdinPipe() io.WriteCloser
	StdoutPipe() io.ReadCloser
	StderrPipe() io.ReadCloser
	// SetSize will set the TTY size, returns ErrFeatureNotSupported, if the
	// shell wasn't launched as a TTY, or platform doesn't support size options.
	//
	// non-fatal errors: ErrShellTerminated, ErrShellAborted,
	// ErrFeatureNotSupported
	SetSize(columns, rows uint16) error
	// Aborts a shell, causing Wait() to return ErrShellAborted. If the shell has
	// already terminated Abort() returns ErrShellTerminated.
	//
	// non-fatal errors: ErrShellTerminated
	Abort() error
	// Wait will return when the shell has terminated. It returns true/false
	// depending on the exit code. Any error indicates that the shell didn't
	// run in a controlled maner. If Abort() was called Wait() shall return
	// ErrShellAborted.
	//
	// non-fatal errors: ErrShellAborted
	Wait() (bool, error)
}

// The Display struct holds information about a display that exists inside
// a running sandbox.
type Display struct {
	Name        string
	Description string
	Width       int // 0 if unknown
	Height      int // 0 if unknown
}

// The Sandbox interface represents an active sandbox.
//
// All methods on this interface must be thread-safe.
type Sandbox interface {
	// Wait for task execution and termination of all associated shells, and
	// return immediately if sandbox execution has finished.
	//
	// When this method returns, all resources held by the Sandbox instance must
	// have been released or transferred to the ResultSet instance returned. If an
	// internal error occured, resources may be freed and WaitForResult() may
	// return ErrNonFatalInternalError if the error didn't leak resources and we
	// don't expect the error to be persistent.
	//
	// When this method has returned, any calls to Abort() or NewShell() should
	// return ErrSandboxTerminated. If Abort() is called before WaitForResult()
	// returns, WaitForResult() should return ErrSandboxAborted and release all
	// resources held.
	//
	// Notice that this method may be invoked more than once. In all cases it
	// should return the same value when it decides to return. In particular, it
	// must keep a reference to the ResultSet instance created and return the same
	// instance, so that any resources held aren't transferred to multiple
	// different ResultSet instances.
	//
	// Non-fatal errors: ErrNonFatalInternalError, ErrSandboxAborted.
	WaitForResult() (ResultSet, error)

	// NewShell creates a new Shell for interaction with the sandbox. The shell
	// and arguments to be launched can be specified with command, if no command
	// arguments are given the sandbox should create a shell of the platforms
	// default type.
	//
	// If the engine doesn't support interactive shells it may return
	// ErrFeatureNotSupported. This should not interrupt/abort the execution of
	// the task which should proceed as normal.
	//
	// If the WaitForResult() method has returned and the sandbox isn't running
	// anymore this method must return ErrSandboxTerminated, signaling that you
	// can't interact with the sandbox anymore.
	//
	// Non-fatal errors: ErrFeatureNotSupported, ErrSandboxTerminated.
	NewShell(command []string, tty bool) (Shell, error)

	// ListDisplays returns a list of Display objects that describes displays
	// that exists inside the Sandbox while it's running.
	//
	// Non-fatal errors: ErrFeatureNotSupported, ErrSandboxTerminated.
	ListDisplays() ([]Display, error)

	// OpenDisplay returns an active VNC connection to a display with the given
	// name inside the running Sandbox.
	//
	// If no such display exist within the sandbox this method should return:
	// ErrNoSuchDisplay.
	//
	// Non-fatal errors: ErrFeatureNotSupported, ErrNoSuchDisplay,
	// ErrSandboxTerminated.
	OpenDisplay(name string) (io.ReadWriteCloser, error)

	// Abort the sandbox. This means killing the task execution as well as all
	// associated shells and releasing all resources held.
	//
	// If called before the sandbox execution finished, then WaitForResult() must
	// return ErrSandboxAborted. If sandbox execution has finished when Abort() is
	// called, Abort() should return ErrSandboxTerminated and not release any
	// resources as they should have been released by WaitForResult() or
	// transferred to the ResultSet instance returned.
	//
	// Non-fatal errors: ErrSandboxTerminated
	Abort() error
}

// SandboxBase is a base implemenation of Sandbox. It will implement all
// optional methods such that they return ErrFeatureNotSupported.
//
// Note: This will not implement WaitForResult() and other required methods.
//
// Implementors of SandBox should embed this struct to ensure source
// compatibility when we add more optional methods to SandBox.
type SandboxBase struct{}

// NewShell returns ErrFeatureNotSupported indicating that the feature isn't
// supported.
func (SandboxBase) NewShell(command []string, tty bool) (Shell, error) {
	return nil, ErrFeatureNotSupported
}

// ListDisplays returns ErrFeatureNotSupported indicating that the feature isn't
// supported.
func (SandboxBase) ListDisplays() ([]Display, error) {
	return nil, ErrFeatureNotSupported
}

// OpenDisplay returns ErrFeatureNotSupported indicating that the feature isn't
// supported.
func (SandboxBase) OpenDisplay(string) (io.ReadWriteCloser, error) {
	return nil, ErrFeatureNotSupported
}

// Abort returns nil indicating that resources have been released.
func (SandboxBase) Abort() error {
	return nil
}
