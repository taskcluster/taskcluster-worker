package plugins

import (
	"github.com/taskcluster/taskcluster-worker/engine"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

// PluginProvider holds the global state for a plugin.
//
// All method on this interface must be thread-safe.
type PluginProvider interface {
	// NewPlugin method will be called once for each task. The Plugin instance
	// returned will be called for each stage in the task execution.
	//
	// This method is a called before PrepareSandbox(), this is not a good place
	// to do any operation that may fail as you won't be able to log anything.
	// This is, however, the place to register things that you wish to expose to
	// engine and other plugins, such as a log drain.
	NewPlugin(builder *runtime.SandboxContextBuilder) Plugin
}

// Plugin holds the task-specific state for a plugin
//
// Each method on this interface represents stage in the task execution and will
// be called when this stage is reached. The methods are allowed allowed to
// take signaficant amounts of time, as they will run asynchronously.
//
// These methods does not have to be thread-safe, we will not call the next
// method, before the previous method has returned.
//
// If a required feature is unsupport the methods may return a
// MalformedPayloadError. All other errors are fatal.
type Plugin interface {
	// Prepare will be called in parallel with PrepareSandbox(), this is a good
	// place to start downloading and extracting resources required.
	//
	// Notice that this method may in fact do long running operations. It will
	// run in parallel with PrepareSandbox(), so if that is loading a docker image
	// you may take your time here.
	Prepare(context *runtime.SandboxContext) error
	// Prepared is called once PrepareSandbox() has returned.
	//
	// This is the place to mount caches, proxies, etc.
	//
	// Non-fatal errors: MalformedPayloadError
	Prepared(preparedSandbox engine.PreparedSandbox) error
	// Started is called once the sandbox has started execution. This is a good
	// place to hook if you want to do interactive things.
	//
	// Non-fatal errors: MalformedPayloadError
	Started(sandbox engine.Sandbox) error
	// Stopped is called once the sandbox has terminated. Returns true, if the
	// task execution was successful.
	//
	// This is a good place to upload artifacts, logs, check exit code, and start
	// to cleanup resources used for interactive features.
	//
	// Non-fatal errors: MalformedPayloadError
	Stopped(result engine.ResultSet) (bool, error)
	// Dispose is called once the task is done and we have decided not proceed.
	//
	// This method may be invoked at anytime, it is then the responsibility of the
	// of the Plugin implementation to clean up any resources held. Notice that,
	// that some stages might be skipped if the engine or another plugin aborts
	// task execution due to malformed-payload or internal error.
	//
	// Non-fatal errors: MalformedPayloadError
	Dispose() error
}

// PluginBase is a base implementation of the plugin interface, it just handles
// all methods and does nothing. If you embed this you only have to implement
// the methods you care about.
type PluginBase struct{}

// Prepare ignores the sandbox preparation stage.
func (PluginBase) Prepare(*runtime.SandboxContext) error {
	return nil
}

// Prepared ignores the sandbox preparation stage.
func (PluginBase) Prepared(engine.PreparedSandbox) error {
	return nil
}

// Started ignores the stage where the sandbox has started
func (PluginBase) Started(engine.Sandbox) error {
	return nil
}

// Stopped ignores the stage where the sandbox has returned a ResultSet, and
// returns true saying the task was successful, as not to poison the water.
func (PluginBase) Stopped(engine.ResultSet) (bool, error) {
	return true, nil
}

// Dispose ignores the stage where resources are disposed.
func (PluginBase) Dispose() error {
	return nil
}
