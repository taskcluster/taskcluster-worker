package engine

import (
	"github.com/taskcluster/taskcluster-worker/engine/mock"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

// An Engine implementation provides and backend upon which tasks can be
// executed.
//
// We do not intend for a worker to use multiple engines at the same time,
// whilst it might be fun to try some day, implementors need not design with
// this use-case in mind. This means that you can safely assume that your
// engine is the only engine that is instantiated.
//
// While we do not intend to use multiple engines at the same time, implementors
// must design engines to support running multiple sandboxes in parallel. If
// the engine can't run multiple sandboxes in parallel, it should return set
// IsSingletonEngine to false in its FeatureSet(). Additionally, it must return
// ErrEngineIsSingleton if a second sandbox is created, before the previous
// sandbox is disposed.
//
// Obviously not all engines are available on all platforms and not all features
// can be implemented on all platforms. See individual methods to see which are
// required and which can be implemented by returning ErrFeatureNotSupported.
type Engine interface {
	// FeatureSet returns a structure declaring which features are supported, this
	// is used for feature checking. Though most methods only needs to return
	// ErrFeatureNotSupported when called, rather than being declared here.
	FeatureSet() FeatureSet
	// PrepareSandbox returns a new instance of the PrepareSandbox interface.
	//
	// We'll create a PrepareSandbox for each task run. This is really a setup
	// step where the implementor may acquire resources referenced in the
	// SandboxOptions..
	//
	// Example: An engine implementation based on docker, may download the docker
	// image in before returning from SandboxOptions(). The PreparedSandbox
	// instance returned will then reference the docker image, and be ready to
	// start a new docker container once StartSandbox() is called.
	//
	// This operation should parse the engine-specific payload parts given in
	// SandboxOptions and return a MalformedPayloadError error if the payload
	// is invalid.
	//
	// Non-fatal errors: MalformedPayloadError, ErrEngineIsSingleton.
	PrepareSandbox(options *SandboxOptions, context *runtime.SandboxContext) (PreparedSandbox, error)
	// NewCacheFolder returns a new Volume backed by a file system folder
	// if cache-folders folders are supported, otherwise it must return
	// ErrFeatureNotSupported.
	//
	// Non-fatal errors: ErrFeatureNotSupported
	NewCacheFolder() (Volume, error)
	// NewMemoryDisk returns a new Volume backed by a ramdisk, if ramdisks are
	// supported, otherwise it must return ErrFeatureNotSupported.
	//
	// Non-fatal errors: ErrFeatureNotSupported
	NewMemoryDisk() (Volume, error)
}

// The FeatureSet structure defines the set of features supported by an engine.
//
// Some plugins will use this for feature detection, most plugins will call the
// methods in question and handle the ErrFeatureNotSupported error. For this
// reason it's essential to also return ErrFeatureNotSupported from methods
// related to unsupported features (see docs of individual methods).
//
// Plugin implementors are advised to call methods and handling unsupported
// features by handling ErrFeatureNotSupported errors. But in some cases it
// might be necessary to adjust behavior in case of unsupported methods, for
// this upfront feature checking using FeatureSet is necessary.
//
// To encourage the try and handle errors pattern, the FeatureSet shall only
// list features for which we critically need upfront feature testing.
type FeatureSet struct {
	IsSingletonEngine bool
}

// NewEngine returns an engine implementation, if the requested engine is
// available under the current build contraints, GOOS, GOARCH.
//
// This function is intended to be called once immediately after startup, with
// engine of choice as given by configuration.
func NewEngine(engineName string, runtime *runtime.EngineContext) (Engine, error) {
	if platform == "mock" {
		return mock.NewMockEngine(runtime)
	}
	return nil, ErrEngineUnknown
}
