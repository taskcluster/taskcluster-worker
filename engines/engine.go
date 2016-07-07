package engines

import "github.com/taskcluster/taskcluster-worker/runtime"

// The SandboxOptions structure is a wrapper around the options/arguments for
// creating a NewSandboxBuilder. This allows us to add new arguments without
// breaking source compatibility with older Engine implementations.
type SandboxOptions struct {
	// TaskContext contains information about the task we're starting a sandbox
	// for.
	TaskContext *runtime.TaskContext
	// Result from PayloadSchema().Parse(). Implementors are safe to assert
	// this back to their target type.
	Payload interface{}
}

// An Engine implementation provides a backend upon which tasks can be
// executed.
//
// We do not intend for a worker to use multiple engines at the same time,
// whilst it might be fun to try some day, implementors need not design with
// this use-case in mind. This means that you can safely assume that your
// engine is the only engine that is instantiated.
//
// While we do not intend to use multiple engines at the same time, implementors
// must design engines to support running multiple sandboxes in parallel. If
// the engine can't run an unbounded number of sandboxes in parallel, it should
// return set MaxConcurrency to non-zero in its Capabilities(). Additionally,
// it must return ErrMaxConcurrencyExceeded if a sandbox is creation would
// violate it's declared MaxConcurrency. Obviously, when a sandbox is disposed
// it should be possible call NewSandboxBuilder() again.
//
// Obviously not all engines are available on all platforms and not all features
// can be implemented on all platforms. See individual methods to see which are
// required and which can be implemented by returning ErrFeatureNotSupported.
type Engine interface {
	// PayloadSchema returns the CompositeSchema that represents the payload.
	//
	// The Payload property on SandboxOptions given to NewSandboxBuilder will be
	// the result from CompositeSchema.Parse() on the CompositeSchema returned
	// from this method.
	PayloadSchema() runtime.CompositeSchema

	// Capabilities returns a structure declaring which features are supported,
	// this is used for up-front feature checking. Unsupport methods must also
	// return ErrFeatureNotSupported when called.
	//
	// This property is strictly for plugins that need to do up-front feature
	// checking. Consumers are encouraged to just try them and handle errors
	// rather than testing for supported features up-front. Granted this is not
	// always possible, hence, the presence of this property.
	//
	// Implementors must return a constant that is always the same.
	Capabilities() Capabilities

	// NewSandboxBuilder returns a new instance of the SandboxBuilder interface.
	//
	// We'll create a SandboxBuilder for each task run. This is really a setup
	// step where the implementor may acquire resources referenced in the
	// SandboxOptions.
	//
	// Example: An engine implementation based on docker, may start downloading
	// the docker image in before returning from NewSandboxBuilder(). The
	// SandboxBuilder instance returned will then reference the docker image
	// downloading process, and be ready to start a new docker container once
	// StartSandbox() is called. Obviously blocking that call until docker image
	// download is completed.
	//
	// This operation should parse the engine-specific payload parts given in
	// SandboxOptions and return a MalformedPayloadError error if the payload
	// is invalid.
	//
	// Non-fatal errors: MalformedPayloadError, ErrMaxConcurrencyExceeded.
	NewSandboxBuilder(options SandboxOptions) (SandboxBuilder, error)

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

	// Dispose cleans up any resources held by the engine. The engine object
	// cannot be used after Dispose() has been called.
	//
	// This method need not be thread-safe! And may NOT be called before all
	// SandboxBuilders, Sandboxes and ResultSets have been disposed.
	//
	// This is mostly useful for cleanup after tests, as we won't switch between
	// engines in production.
	Dispose() error
}

// The Capabilities structure defines the set of features supported by an engine.
//
// Some plugins will use this for feature detection, most plugins will call the
// methods in question and handle the ErrFeatureNotSupported error. For this
// reason it's essential to also return ErrFeatureNotSupported from methods
// related to unsupported features (see docs of individual methods).
//
// Plugin implementors are advised to call methods and handling unsupported
// features by handling ErrFeatureNotSupported errors. But in some cases it
// might be necessary to adjust behavior in case of unsupported methods, for
// this up-front feature checking using Capabilities is necessary.
//
// To encourage the try and handle errors pattern, the Capabilities shall only
// list features for which we critically need up-front feature testing.
type Capabilities struct {
	// Maximum number of parallel sandboxes, leave 0 if unbounded.
	MaxConcurrency int
	// Note: the zero value of Capabilities should always indicate the sane
	// defaults, typically that a feature isn't supported.
}

// EngineBase is a base implemenation of Engine. It will implement all optional
// methods such that they return ErrFeatureNotSupported.
//
// Note: This will not implement NewSandboxBuilder() and other required methods.
//
// Implementors of Engine should embed this struct to ensure source
// compatibility when we add more optional methods to Engine.
type EngineBase struct{}

// PayloadSchema returns an empty CompositeSchema indicating that a nil
// payload is sufficient.
func (EngineBase) PayloadSchema() runtime.CompositeSchema {
	return runtime.NewEmptyCompositeSchema()
}

// ConfigSchema returns an empty jsonschema indicating that no custom config is
// required.
func (EngineBase) ConfigSchema() runtime.CompositeSchema {
	return runtime.NewEmptyCompositeSchema()
}

// Capabilities returns an zero value Capabilities struct indicating that
// most features aren't supported.
func (EngineBase) Capabilities() Capabilities {
	return Capabilities{}
}

// NewCacheFolder returns ErrFeatureNotSupported indicating that the feature
// isn't supported.
func (EngineBase) NewCacheFolder() (Volume, error) {
	return nil, ErrFeatureNotSupported
}

// NewMemoryDisk returns ErrFeatureNotSupported indicating that the feature
// isn't supported.
func (EngineBase) NewMemoryDisk() (Volume, error) {
	return nil, ErrFeatureNotSupported
}

// Dispose trivially implements cleanup by doing nothing.
func (EngineBase) Dispose() error {
	return nil
}
