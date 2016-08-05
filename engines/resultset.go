package engines

import (
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

// FileHandler is given as callback when iterating through a list of files.
//
// ResultSet.ExtractFolder(path, handler) takes a FileHandler as the handler
// parameter. This function maybe called sequentially or concurrently, but if
// it returns an the ResultSet should stop calling it and pass the error through
// as return value from ResultSet.ExtractFolder.
type FileHandler func(path string, stream ioext.ReadSeekCloser) error

// The ResultSet interface represents the results of a sandbox that has finished
// execution, but is hanging around while results are being extracted.
//
// When returned from Sandbox this takes ownership of all resources. If the
// engine uses docker then the ResultSet would have ownership of cache folders
// as well as the terminated docker container.
//
// All methods on this interface must be thread-safe.
type ResultSet interface {
	// Success returns true if the execution was successful, typically implying
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
	ExtractFile(path string) (ioext.ReadSeekCloser, error)

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
	// For each file found under the given path the handler(path, stream) is
	// called. Implementor may call this function sequentially or in parallel.
	// If a handler(path, stream) call returns an error then ErrHandlerInterrupt
	// should be passed as return value from the ExtractFolder call.
	//
	// If an error occurs during iteration, iteration is halted, and when all
	// calls to handler(path, stream) have returned, ExtractFolder should return
	// with ErrHandlerInterrupt.
	//
	// The only non-fatal error is ErrNonFatalInternalError, indicating that
	// something went wrong while streaming out artfacts and all artifacts may not
	// have been extracted, or they may not have been streamed out completely.
	//
	// The ErrNonFatalInternalError may only be returned if the engine expected
	// further request to be successful. And attempts to call other methods or
	// extract other paths might work out fine.
	//
	// Non-fatal erorrs: ErrFeatureNotSupported, ErrResourceNotFound,
	// MalformedPayloadError, ErrNonFatalInternalError, ErrHandlerInterrupt
	ExtractFolder(path string, handler FileHandler) error

	// ArchiveSandbox streams out the entire sandbox (or as much as possible)
	// as a tar-stream. Ideally this also includes cache folders.
	ArchiveSandbox() (ioext.ReadSeekCloser, error)

	// Dispose shall release all resources.
	//
	// CacheFolders given to the sandbox shall not be disposed, instead they are
	// just no longer owned by the engine.
	//
	// Implementors should only return an error if cleaning up fails and the
	// worker therefor needs to stop operation.
	Dispose() error
}

// ResultSetBase is a base implemenation of ResultSet. It will implement all
// optional methods such that they return ErrFeatureNotSupported.
//
// Note: This will not implement Success() and other required methods.
//
// Implementors of ResultSet should embed this struct to ensure source
// compatibility when we add more optional methods to ResultSet.
type ResultSetBase struct{}

// ExtractFile returns ErrFeatureNotSupported indicating that the feature isn't
// supported.
func (ResultSetBase) ExtractFile(string) (ioext.ReadSeekCloser, error) {
	return nil, ErrFeatureNotSupported
}

// ExtractFolder returns ErrFeatureNotSupported indicating that the feature
// isn't supported.
func (ResultSetBase) ExtractFolder(string, FileHandler) error {
	return ErrFeatureNotSupported
}

// ArchiveSandbox returns ErrFeatureNotSupported indicating that the feature
// isn't supported.
func (ResultSetBase) ArchiveSandbox() (ioext.ReadSeekCloser, error) {
	return nil, ErrFeatureNotSupported
}

// Dispose returns nil indicating that resources have been released.
func (ResultSetBase) Dispose() error {
	return nil
}
