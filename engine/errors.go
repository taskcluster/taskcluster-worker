package engine

import "errors"

var (
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
	ErrFeatureNotSupported   = errors.New("Feature not support by current engine")
	ErrResourceNotFound      = errors.New("The referenced resource wasn't found")
	ErrSandboxTerminated     = errors.new("The Sandbox has terminated")
	ErrSandboxAborted        = errors.new("Exection of sandbox was aborted")
	ErrNonFatalInternalError = errors.new("Engine encountered a non-fatal internal error")
	ErrContractViolation     = errors.new("Engine has detected a contract violation")
	ErrEngineIsSingleton     = errors.New("Engine cannot run multiple sandboxes in parallel")
	ErrEngineNotSupported    = errors.New("Engine is not available in the current configuration")
	ErrEngineUnknown         = errors.New("Engine with the given doesn't exist")
)

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

func (e *MalformedPayloadError) Error() string {
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
