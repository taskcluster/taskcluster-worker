package runtime

import "errors"

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
