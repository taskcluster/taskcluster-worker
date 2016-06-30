package engines

import (
	"errors"
	"fmt"
)

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
var ErrFeatureNotSupported = errors.New("Feature not supported by the current engine")

// ErrMutableMountNotSupported is returned when volume attachments are
// supported, but mutable mounts aren't supported.
var ErrMutableMountNotSupported = errors.New("The engine doesn't support mutable volume attachments")

// ErrImmutableMountNotSupported is returned when volume attachements are
// supported, but immutable mounts aren't supported.
var ErrImmutableMountNotSupported = errors.New("The engine doesn't support immutable volume attachements")

// ErrSandboxBuilderDiscarded is returned when a SandboxBuilder was discarded
// while StartSandbox() was in the process of starting the sandbox.
var ErrSandboxBuilderDiscarded = errors.New("The SandboxBuilder was discarded while StartSandbox() was running")

// ErrResourceNotFound is returned when trying to extract a file or folder that
// doesn't exist.
var ErrResourceNotFound = errors.New("The referenced resource wasn't found")

// ErrHandlerInterrupt is returned when a handler that was given returns an error
var ErrHandlerInterrupt = errors.New("Handler returned an error and interrupted iteration")

// ErrSandboxTerminated is used to indicate that a SandBox has already
// terminated and can't be aborted.
var ErrSandboxTerminated = errors.New("The Sandbox has terminated")

// ErrSandboxAborted is used to indicate that a Sandbox has been aborted.
var ErrSandboxAborted = errors.New("Exection of sandbox was aborted")

// ErrShellTerminated is used to indicate that a shell has already terminated
var ErrShellTerminated = errors.New("The shell has already terminated")

// ErrShellAborted is used to indicate that a Shell has been aborted.
var ErrShellAborted = errors.New("The shell was aborted")

// ErrNoSuchDisplay is used to indicate that a requested display doesn't exist.
var ErrNoSuchDisplay = errors.New("No such display exists")

// ErrNamingConflict is used to indicate that a name is already in use.
var ErrNamingConflict = errors.New("Conflicting name is already in use")

// ErrNonFatalInternalError is used to indicate that the operation failed
// because of internal error that isn't expected to affect other tasks.
var ErrNonFatalInternalError = errors.New("Engine encountered a non-fatal internal error")

// ErrContractViolation is returned when a contract with the engine has been
// violated.
var ErrContractViolation = errors.New("Engine has detected a contract violation")

// ErrMaxConcurrencyExceeded is returned when the engine has limitation on how
// many sandboxes it can run in parallel and this limit is violated.
var ErrMaxConcurrencyExceeded = errors.New("Engine is cannot run more than " +
	"Engine.Capabilities().MaxCurrency sandbox in parallel")

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
	messages []string
}

// Error returns the error message and adheres to the Error interface
func (e MalformedPayloadError) Error() string {
	if len(e.messages) == 1 {
		return e.messages[0]
	}
	//TODO: Make this smarter in some way please!
	msg := ""
	for _, m := range e.messages {
		msg += m + "\n"
	}
	return msg
}

// NewMalformedPayloadError creates a MalformedPayloadError object, please
// make sure to include a detailed description of the error, preferably using
// multiple lines and with examples.
//
// These will be printed in the logs and end-users will rely on them to debug
// their tasks.
func NewMalformedPayloadError(a ...interface{}) MalformedPayloadError {
	return MalformedPayloadError{messages: []string{fmt.Sprint(a...)}}
}

// MergeMalformedPayload merges a list of MalformedPayloadError objects
func MergeMalformedPayload(errors ...MalformedPayloadError) MalformedPayloadError {
	messages := []string{}
	for _, e := range errors {
		messages = append(messages, e.messages...)
	}
	return MalformedPayloadError{messages: messages}
}

// InternalError are errors that could not be completed because of issues related to the
// host.  These issues could include issues with the engine, host resources, and worker
// configuration.
type InternalError struct {
	message string
}

// Error returns the error message and adheres to the Error interface
func (e InternalError) Error() string {
	return e.message
}

// NewInternalError creates an InternalError object, please
// make sure to include a detailed description of the error, preferably using
// multiple lines and with examples.
//
// These will be printed in the logs and end-users will rely on them to debug
// their tasks.
func NewInternalError(message string) InternalError {
	return InternalError{message: message}
}
