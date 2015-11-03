package plugins

import (
	"github.com/taskcluster/taskcluster-worker/engine"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

// PluginFactory holds the global state for a plugin.
//
// All method on this interface must be thread-safe.
type PluginFactory interface {
	// NewPlugin method will be called once for each task. The Plugin instance
	// returned will be called for each stage in the task execution.
	//
	// This method is a called before NewSandboxBuilder(), this is not a good
	// place to do any operation that may fail as you won't be able to log
	// anything. This is, however, the place to register things that you wish to
	// expose to engine and other plugins, such as a log drain.
	//
	// Notice, if for some reason the implementor doesn't wish to return a plugin
	// perhaps runtime.SandboxContextBuilder contains task information for that
	// disables the plugin, the implementor can safely return an instance of
	// PluginBase. Such and instance will do absolutely nothing.
	NewPlugin(builder *runtime.SandboxContextBuilder) Plugin
}

// Plugin holds the task-specific state for a plugin
//
// Each method on this interface represents stage in the task execution and will
// be called when this stage is reached. The methods are allowed allowed to
// take significant amounts of time, as they will run asynchronously.
//
// These methods does not have to be thread-safe, we will not call the next
// method, before the previous method has returned.
//
// implementors of this interface should be sure to embed PluginBase. This will
// do absolutely nothing, but provide empty implementations for any current and
// future methods that isn't implemented.
//
// If a required feature is unsupport the methods may return a
// MalformedPayloadError. All other errors are fatal.
type Plugin interface {
	// Prepare will be called in parallel with NewSandboxBuilder(), this is a good
	// place to start downloading and extracting resources required.
	//
	// Notice that this method is a good place to start long-running operations,
	// you then have to take care to clean-up in dispose or wait for the
	// long-running operations to finish in BuildSandbox().
	Prepare(context *runtime.SandboxContext) error
	// BuildSandbox is called once NewSandboxBuilder() has returned.
	//
	// This is the place to wait for downloads and other expensive operations to
	// finished, before mounting caches, proxies, etc. and returning.
	//
	// Non-fatal errors: MalformedPayloadError
	BuildSandbox(SandboxBuilder engine.SandboxBuilder) error
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

// PluginFactoryBase is a base implementation of the PluginFactory interface,
// it just returns a Plugin instance with empty methods (PluginBase).
//
// Plugin implementor may return this from your NewXXXPluginFactory() method,
// if it based on the engine given is determined that the Plugin should be
// disabled for the life-cycle of this worker.
type PluginFactoryBase struct{}

// NewPlugin returns a PluginBase with empty methods.
func (PluginFactoryBase) NewPlugin(*runtime.SandboxContextBuilder) Plugin {
	return PluginBase{}
}

// PluginBase is a base implementation of the plugin interface, it just handles
// all methods and does nothing. If you embed this you only have to implement
// the methods you care about.
type PluginBase struct{}

// Prepare ignores the sandbox preparation stage.
func (PluginBase) Prepare(*runtime.SandboxContext) error {
	return nil
}

// BuildSandbox ignores the sandbox building stage.
func (PluginBase) BuildSandbox(engine.SandboxBuilder) error {
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
