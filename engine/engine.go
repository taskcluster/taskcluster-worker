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
	// TODO: Consider giving plugin and engine a method:
	// schema() []SchemaEntry
	// type SchemaEntry struct {
	// 		property	string,
	//    schema 		string,
	//    required: bool,
	//    target:  	interface{}
	// }
	// Then an engine can defined whatever properties it wants, and whether they
	// are required. As well as the schema they must specify.
	//
	// Now the runtime can:
	//  - publish a payload schema for the current configuration
	//  - validate the JSON input before giving it to the engine
	//  - parse JSON into target before giving it to engine
	//  - guard against plugins and engines relying on the same properties
	//		(or require that if they do, they declare the same schema and target)
	//
	// There is two downsides to this:
	//  - All of these checks happens at runtime, no compile-time error
	//		This is likely acceptable, as we want to run "worker --payload-schema"
	//    to print the payload schema before we deploy a worker. Hence, conflicts
	//		would be reported here.
	//    Well, some won't be caught like target not working json.Unmarshal, but
	//    hopefully that would be caught in testing. Otherwise it's a pretty
	//    obvious bug.
	//  - JSON schema is contructed at runtime, so validator can't be generated at
	//    compile-time, however, there is no golang JSON schema that can render a
	//    a JSON schema to code yet. And using gojsonschema is probably fine, it
	//    can do JSON schema validator generation at compile-time anyways.
	//    This is probably not important as performance of schema validation is a
	//    minor concern (after all we only validate a tiny bit of JSON).
	//  - two plugins sharing the same property would be hard, so we can't nest
	//		properties in objects because we think they are related.
	//		(this is probably good, as nesting is sort of a JSON anti-pattern)
	//    But for something like task.payload.mounts = [...], maybe we would want
	//    different plugins handling different mount types, or maybe we just make
	// 		a new plugin abstraction for MountPlugin, or we require all the plugins
	//    to declare the same SchemaEntry, this seems reasonable as we can just
	//    shared that part of the code between them. And do hacks like that.

	// FeatureSet returns a structure declaring which features are supported, this
	// is used for feature checking. Though most methods only needs to return
	// ErrFeatureNotSupported when called, rather than being declared here.
	FeatureSet() FeatureSet
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
	// StartSandbox() is called, Obviously blocking that call until docker image
	// download is completed.
	//
	// This operation should parse the engine-specific payload parts given in
	// SandboxOptions and return a MalformedPayloadError error if the payload
	// is invalid.
	//
	// Non-fatal errors: MalformedPayloadError, ErrEngineIsSingleton.
	NewSandboxBuilder(options *SandboxOptions, context *runtime.SandboxContext) (SandboxBuilder, error)
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
	if engineName == "mock" {
		return mock.NewMockEngine(runtime)
	}
	return nil, ErrEngineUnknown
}
