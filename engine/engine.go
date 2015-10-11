package engine

import (
	"github.com/taskcluster/taskcluster-worker/engine/mock"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

// An Engine implementation provides and backend upon which tasks can be
// executed. We do not intend for a worker to use multiple engines in parallel,
// whilst it might be fun to try some day, you should design engines for this
// purpose, nor expect this functionality.
//
// Obviously not all engines are available on all platforms and not all features
// can be implemented on all platforms. See individual methods to see which are
// required and which can be implemented by returning ErrFeatureNotSupported.
type Engine interface {
	// NewSandbox returns a new instance of the Sandbox interface. We'll create
	// a Sandbox for each task run. Hence, the Sandbox may be stateful.
	//
	// This operation should parse the task-specific payload and return a
	// MalformedPayloadError error if the payload isn't valid.
	NewSandbox(payload *SandboxPayload, context *runtime.SandboxContext) (Sandbox, error)
	// NewCacheFolder returns a new CacheFolder, if CacheFolder folders are
	// supported, otherwise it may return ErrFeatureNotSupported without causing
	// a panic (any other error will cause the worker to panic)
	NewCacheFolder() (CacheFolder, error)
}

// NewEngine returns an engine implementation or nil, if the requested engine
// isn't available under the current build contraints, GOOS, GOARCH, or if the
// engine simplify doesn't exist.
//
// This function is intended to be called once immediately after startup, with
// engine of choice as given by configuration.
func NewEngine(engineName string, runtime *runtime.EngineContext) Engine {
	if platform == "mock" {
		return mock.NewMockEngine(runtime)
	}
	return nil
}
