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
