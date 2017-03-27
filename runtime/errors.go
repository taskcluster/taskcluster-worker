package runtime

import (
	"errors"
	"fmt"
	"strings"
)

// ErrNonFatalInternalError is used to indicate that the operation failed
// because of internal error that isn't expected to affect other tasks.
//
// Worker need not worry about logging the error to system log or task log as
// the engine/plugin which returned this error already reported it, log it
// and/or deemed the error inconsequential.
//
// Worker should, however, report the task as exception and resolve it with
// reason 'internal-error'. If the worker gets a lot of these non-fatal internal
// errors, it may employ a heuristic to decide if it has entered a bad state.
// For example, worker might reboot if it has seen more than 5 non-fatal
// internal errors within the span of 15min or 5 tasks.
var ErrNonFatalInternalError = errors.New("Encountered a non-fatal internal error")

// ErrFatalInternalError is used to signal that a fatal internal error has
// been logged and that the worker should gracefully terminate/reset.
//
// Engines and plugins can return any unknown error in-order to trigger the same
// effect. As the worker will report, log and terminate/reset when it encounters
// an unknown error. This error is ONLY used when the error has already been
// reported and logged to both system log and task log.
//
// This is only useful for plugins and engines that wishes to manually handle
// error reporting.
var ErrFatalInternalError = errors.New("Encountered a fatal internal error")

// The MalformedPayloadError error type is used to indicate that some operation
// failed because of malformed-payload.
//
// For example a string expected to be path contained invalid characters, a
// required property was missing, or an integer was outside the permitted range.
type MalformedPayloadError struct {
	messages []string
}

// Error returns the error message and adheres to the Error interface
func (e MalformedPayloadError) Error() string {
	return fmt.Sprintf("malformed-payload error: %s", strings.Join(e.messages, "\n"))
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

// IsMalformedPayloadError casts error to MalformedPayloadError.
//
// This is mostly because it's hard to remember that error isn't supposed to be
// cast to *MalformedPayloadError.
func IsMalformedPayloadError(err error) (e MalformedPayloadError, ok bool) {
	e, ok = err.(MalformedPayloadError)
	return
}
