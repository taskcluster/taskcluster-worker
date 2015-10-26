package engine

import (
	"encoding/json"
	"io"
	"net/http"
)

// The SandboxOptions structure contains the engine specific parts of the
// task payload, and other things an engine may need to prepare a sandbox.
type SandboxOptions struct {
	// The task.payload.start property is engine specific and we leave to the
	// engine implementor to parse it.
	Start json.RawMessage
	// The task.payload.options property is engine specific and we leave to the
	// engine implementor to parse it.
	Options json.RawMessage
}

//TODO: Rename PreparedSandbox to SandboxBuilder

// The PreparedSandbox interface wraps the state required to start a Sandbox.
//
// Before returning a PreparedSandbox engine implementors should download and
// setup all the resources needed to start execution. A docker based engine may
// wish to ensure the docker image is downloaded, and lay a claim on it so the
// GarbageCollector won't remove it. A naive Windows engine may wish to create
// a new user account and setup a folder for the sandbox.
//
// Implementors can be sure that any instance of this interface will only be
// called once. That is either StartSandbox() or Abort() will be called, and
// only ever once. If StartSandbox() is called twice a sane implementor should
// return ErrContractViolation, or feel free to exhibit undefined behavior.
type PreparedSandbox interface {
	AttachVolume(mountpoint string, volume Volume, readOnly bool) error
	AttachProxy(name string, handler http.Handler) error
	// Start execution of task in sandbox. After a call to this method resources
	// held by the PreparedSandbox instance should be released or transferred to
	// the Sandbox implementation.
	//
	// This method may return a MalformedPayloadError if any of the identifiers
	// given in the ExecutionOptions violates engine-specific restrictions. The
	// errors ErrReadOnlyCacheNotSupported and ErrMutableCacheNotSupported, may
	// be returned if the engine doesn't support read-only or mutable caches.
	// The error ErrProxiesNotSupported may be returned if the engine doesn't
	// support proxy attachments.
	//
	// If the method returns an error then it must also free any resources held
	// by the PreparedSandbox implemention. As no method on the PreparedSandbox
	// will be invoked again.
	//
	// Non-fatal errors: MalformedPayloadError, ErrProxiesNotSupported,
	// ErrReadOnlyCacheNotSupported, ErrMutableCacheNotSupported.
	StartSandbox() (Sandbox, error)
	// Abort must free all resources held by the PreparedSandbox interface.
	// Any error returned is fatal, so do not return an error unless there is
	// something very wrong.
	Abort() error
}

// The Shell interface is not fully specified yet, but the idea is that it pipes
// data to a shell inside a Sandbox.
type Shell interface {
	StdinPipe() (io.WriteCloser, error)
	StdoutPipe() (io.ReadCloser, error)
	StderrPipe() (io.ReadCloser, error)
	Terminate() error
	Wait() (bool, error)
}

// The Sandbox interface represents an active sandbox.
type Sandbox interface {
	// Wait for task execution and termination of all associated shells, and
	// return immediately if sandbox execution has finished.
	//
	// When this method returns all resources held by the Sandbox instance must
	// have been released or transferred to the ResultSet instance returned. If an
	// internal error occured resources may be freed and Wait() may return
	// ErrNonFatalInternalError if the error didn't leak resources and we don't
	// expect the error to be persistent.
	//
	// When this method has returned any calls to Abort() or NewShell() should
	// return ErrSandboxTerminated. If Abort() is called before Wait() returns
	// Wait() should return ErrSandboxAborted and release all resources held.
	//
	// Notice that this method may be invoked more than once. In all cases it
	// should return the same value when it decides to return. In particular, it
	// must keep a reference to the ResultSet instance created and return the same
	// instance, so that any resources held aren't transferred to multiple
	// different ResultSet instances.
	//
	// Non-fatal errors: ErrNonFatalInternalError, ErrSandboxAborted.
	Wait() (ResultSet, error)
	// NewShell creates a new Shell for interaction with the sandbox. This is
	// useful for debugging and other purposes.
	//
	// If the engine doesn't support interactive shells it may return
	// ErrFeatureNotSupported. This should not interrupt/abort the execution of
	// the task which should proceed as normal.
	//
	// If the Wait() method has returned and the sandbox isn't running anymore
	// this method must return ErrSandboxTerminated, signaling that you can't
	// interact with the sandbox anymore.
	//
	// Non-fatal errors: ErrFeatureNotSupported, ErrSandboxTerminated.
	NewShell() (Shell, error)
	// Abort the sandbox, this means killing the task execution as well as all
	// associated shells and releasing all resources held.
	//
	// If called before the sandbox execution finished, then Wait() must return
	// ErrSandboxAborted. If sandbox execution has finished when Abort() is called
	// Abort() should return ErrSandboxTerminated and not release any resources
	// as they should have been released by Wait() or transferred to the ResultSet
	// instance returned.
	//
	// Non-fatal errors: ErrSandboxTerminated
	Abort() error
}

// An ArtifactReader contains logic to read an artifact and know it's file path
type ArtifactReader struct {
	// Read/Close the file stream
	io.ReadCloser
	// Path to the artifact within the Sandbox
	Path string
}

// The ResultSet interface represents the results of a sandbox that has finished
// execution, but is hanging around while results are being extracted.
//
// When returned from Sandbox this takes ownership of all resources. If the
// engine uses docker then the ResultSet would have ownership of cache folders
// as well as the terminated docker container.
type ResultSet interface {
	// Success, returns true if the execution was successful, typically implying
	// that the process exited zero.
	Success() bool
	// Extract a file from the sandbox.
	//
	// Interpretation of the string path format is engine specific and must be
	// documented by the engine implementor. The engine may impose restrictions on
	// the string, if these restrictions are violated the engine should return a
	// MalformedPayloadError.
	//
	// If the file requested doesn't exist the engine should return
	// ErrResourceNotFound. Further more the engine may return
	// ErrFeatureNotSupported rather than implementing this method.
	//
	// Non-fatal erorrs: ErrFeatureNotSupported, ErrResourceNotFound,
	// MalformedPayloadError
	ExtractFile(path string) (ArtifactReader, error)
	// Extract a folder from the sandbox.
	//
	// Interpretation of the string path format is engine specific and must be
	// documented by the engine implementor. The engine may impose restrictions on
	// the string, if these restrictions are violated the engine should return a
	// MalformedPayloadError.
	//
	// If the folder requested doesn't exist the engine should return
	// ErrResourceNotFound. Further more the engine may return
	// ErrFeatureNotSupported rather than implementing this method.
	//
	// If no immediate error occurs then ExtractFolder() should returns two
	// channels, a channel over which ArtifactReader structures are transmitted
	// until the channel is closed. And an error channel over which errors can
	// be transmitted, after the ArtifactReader channel is closed.
	//
	// The only non-fatal erorr the error channel can transmit is
	// ErrNonFatalInternalError, indicating that something went wrong while
	// streaming out artfacts and all artifacts may not have been extracted, or
	// they may not have been streamed out completely.
	//
	// The ErrNonFatalInternalError may only be returned if the engine expected
	// further request to be successful. And attempts to call other methods or
	// extract other paths might work out fine.
	//
	// Non-fatal erorrs: ErrFeatureNotSupported, ErrResourceNotFound,
	// MalformedPayloadError, ErrNonFatalInternalError
	ExtractFolder(path string) (<-chan (ArtifactReader), <-chan (error), error)
	// ArchiveSandbox streams out the entire sandbox (or as much as possible)
	// as a tar-stream. Ideally this also includes cache folders.
	ArchiveSandbox() (io.ReadCloser, error)
	// Dispose shall release all resources.
	//
	// CacheFolders given to the sandbox shall not be disposed, instead they are
	// just no longer owned by the engine.
	//
	// Implementors should only return an error if cleaning up fails and the
	// worker therefor needs to stop operation.
	Dispose() error
}
