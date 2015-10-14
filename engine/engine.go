package engine

import (
	"github.com/taskcluster/taskcluster-worker/engine/mock"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

// An Engine implementation provides and backend upon which tasks can be
// executed. We do not intend for a worker to use multiple engines at the same
// time, whilst it might be fun to try some day, implementors should not design
// with this use-case in mind. This means that you can safely assume that your
// engine is the only engine that is instantiated.
//
// While we do not intend to use multiple engines at the same time, implementors
// must design engines to support running multiple sandboxes in parallel. Or
// return the ErrEngineIsSingleton error from PrepareSandbox().
//
// Obviously not all engines are available on all platforms and not all features
// can be implemented on all platforms. See individual methods to see which are
// required and which can be implemented by returning ErrFeatureNotSupported.
type Engine interface {
	// PrepareSandbox returns a new instance of the PrepareSandbox interface.
	// We'll create a PrepareSandbox for each task run. This is really a setup
	// step where the implementor may acquire resources referenced in the
	// PreparationOptions.
	//
	// Example: An engine implementation based on docker, may download the docker
	// image in before returning from PrepareSandbox(). The PreparedSandbox
	// instance returned will then reference the docker image, and be ready to
	// start a new docker container once StartOptions is given to StartSandbox().
	//
	// This operation should parse the engine-specific payload parts given in
	// PreparationOptions and return a MalformedPayloadError error if the payload
	// isn't valid.
	//
	// Non-fatal errors: MalformedPayloadError, ErrEngineIsSingleton.
	PrepareSandbox(options *PreparationOptions, context *runtime.SandboxContext) (PreparedSandbox, error)
	// NewCacheFolder returns a new CacheFolder, if CacheFolder folders are
	// supported, otherwise it may return ErrFeatureNotSupported without causing
	// a panic (any other error will cause the worker to panic)
	//
	// Non-fatal errors: ErrFeatureNotSupported
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
