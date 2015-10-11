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
	ErrFeatureNotSupported = errors.New("Feature not support by current engine")
)

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
