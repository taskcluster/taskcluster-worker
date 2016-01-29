package engines

import "errors"

// ErrFeatureNotSupported is a common error that may be returned from optional
// Engine methods to indicate the engine implementation doesn't support the
// given feature.
//
// Note, all methods are allowed to return this error, some methods are
// required, and may not return this error.
//
// When the worker encounters this error from an optional method, it should
// workaround if possible, but most likely resolve the task as "exception"
// with reason "malformed-payload".
var ErrFeatureNotSupported = errors.New("Feature not support by current engine")

// ErrMutableMountNotSupported is returned when volume attachments are
// supported, but mutable mounts aren't supported.
var ErrMutableMountNotSupported = errors.New("The engine doesn't support mutable volume attachments")

// ErrImmutableMountNotSupported is returned when volume attachements are
// supported, but immutable mounts aren't supported.
var ErrImmutableMountNotSupported = errors.New("The engine doesn't support immutable volume attachements")

// ErrResourceNotFound is returned when trying to extract a file or folder that
// doesn't exist.
var ErrResourceNotFound = errors.New("The referenced resource wasn't found")

// ErrSandboxTerminated is used to indicate that a SandBox has already
// terminated and can't be aborted.
var ErrSandboxTerminated = errors.New("The Sandbox has terminated")

// ErrSandboxAborted is used to indicate that a Sandbox has been aborted.
var ErrSandboxAborted = errors.New("Exection of sandbox was aborted")

// ErrNoSuchDisplay is used to indicate that a requested display doesn't exist.
var ErrNoSuchDisplay = errors.New("No such display exists")

// ErrNonFatalInternalError is used to indicate that the operation failed
// because of internal error that isn't expected to affect other tasks.
var ErrNonFatalInternalError = errors.New("Engine encountered a non-fatal internal error")

// ErrContractViolation is returned when a contract with the engine has been
// violated.
var ErrContractViolation = errors.New("Engine has detected a contract violation")

// ErrEngineIsSingleton is returned when attempts to start multiple sandboxes of
// a singleton engine.
var ErrEngineIsSingleton = errors.New("Engine cannot run multiple sandboxes in parallel")

// ErrEngineNotSupported is used to indicate that the engine isn't supported in
// the current configuration.
var ErrEngineNotSupported = errors.New("Engine is not available in the current configuration")

// TODO: MalformedPayloadError should be define in the runtime
// TODO: MalformedPayloadError should have a merge to join two of these
//       errors (useful if we have multiple of them)

// The MalformedPayloadError error type is used to indicate that some operation
// failed because of malformed-payload. For example a string expected to be
// path contained invalid characters, a required property was missing, or an
// integer was outside the permitted range.
type MalformedPayloadError struct {
	message string
}

func (e MalformedPayloadError) Error() string {
	return e.message
}

// NewMalformedPayloadError creates a MalformedPayloadError object, please
// make sure to include a detailed description of the error, preferably using
// multiple lines and with examples.
//
// These will be printed in the logs and end-users will rely on them to debug
// their tasks.
func NewMalformedPayloadError(message string) MalformedPayloadError {
	return MalformedPayloadError{message: message}
}
