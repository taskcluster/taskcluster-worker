package plugins

import (
	"io"

	"github.com/taskcluster/taskcluster-worker/engine"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

// The PluginOptions is a wrapper for the set of arguments given to NewPlugin.
//
// We wrap the arguments in a single argument to maintain source compatibility
// when introducing additional arguments.
type PluginOptions struct {
	TaskInfo runtime.TaskInfo
	Payload  interface{}
}

// PluginEnvironment holds the definition of a plugin and any global state
// shared between instances of the Plugin.
//
// All method on this interface must be thread-safe.
type PluginEnvironment interface {
	PayloadSchema() runtime.CompositeSchema
	// NewPlugin method will be called once for each task. The Plugin instance
	// returned will be called for each stage in the task execution.
	//
	// This is a poor place to do any processing, and not a great place to start
	// long-running operations as you don't have a place to write log messages.
	// Consider waiting until Prepare() is called with TaskContext that you can
	// write log messages to.
	//
	// Plugins implementing logging should not return an error here, as it
	// naturally follows that such an error can't be logged if no logging plugin
	// is created. Other plugins may also postpone additional payload valdation if
	// they wish to log additional messages in case of errors.
	//
	// Implementors may return nil, if the plugin doesn't have any hooks for the
	// given tasks.
	//
	// Non-fatal errors: MalformedPayloadError
	NewPlugin(options PluginOptions) (Plugin, error)
}

// An ExceptionReason specifies the reason a task reached an exception state.
type ExceptionReason int

// Reasons why a task can reach an exception state. Implementors should be
// warned that additional entries may be added in the future.
const (
	Cancelled ExceptionReason = iota
	MalformedPayload
	WorkerShutdown
)

// Plugin holds the task-specific state for a plugin
//
// Each method on this interface represents stage in the task execution and will
// be called when this stage is reached. The some methods are allowed to
// take significant amounts of time, as they will run asynchronously.
//
// These methods does not have to be thread-safe, we will never call the next
// method, before the previous method has returned.
//
// Implementors of this interface should be sure to embed PluginBase. This will
// do absolutely nothing, but provide empty implementations for any current and
// future methods that isn't implemented.
//
// The methods are called in the order listed here with the exception of
// Exception() which may be called following any method, and Dispose() which
// will always be called as a final step allowing you to clean up.
type Plugin interface {
	// Prepare will be called in parallel with NewSandboxBuilder().
	//
	// Notice that this method is a good place to start long-running operations,
	// you then have to take care to clean-up in Dispose() if they are still
	// running. You should wait for your long-running operations to finish in
	// BuildSandbox() or whatever hook you need them in.
	//
	// Non-fatal errors: MalformedPayloadError
	Prepare(context *runtime.TaskContext) error

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
	// to clean-up resources if such clean-up is expected to take a while.
	//
	// Non-fatal errors: MalformedPayloadError
	Stopped(result engine.ResultSet) (bool, error)

	// Finished is called once the sandbox has terminated and Stopped() have been
	// called.
	//
	// At this stage the task-specific log is closed, and attempts to log data
	// using the previous TaskContext will fail. That makes this a good place to
	// upload logs and any processing of the logs. In fact logging is the primary
	// motivation for this stage.
	//
	// As there is no logging in this method, it's not recommend to do anything
	// that may fail here.
	Finished(success bool) error

	// Exception is called once the task is resolved exception. This may happen
	// instead of calls to Prepare(), BuildSandbox(), Started(), Stopped(), or
	// Finished().
	//
	// This is a good place for best-effort to upload artifacts and logs that you
	// wish to persist. Naturally, log messages written at this stage will be
	// dropped and all error messages will be fatal.
	//
	// Implementors should be ware that additional reasons may be added in the
	// future. Therefore they must handle the default case, if switching on the
	// reason parameter.
	Exception(reason ExceptionReason) error

	// Dispose is called once everything is done and it's time for clean-up.
	//
	// This method will invoked following Stopped() or Exception(). It is then
	// the responsibility of the implementor to abort or wait for any long-running
	// processes and clean-up any resources held.
	Dispose() error
}

// PluginEnvironment is a base implementation of the PluginEnvironment interface.
//
// Implementors should embed this to ensure forward compatibility when we add
// new optional methods.
type PluginEnvironment struct{}

// PayloadSchema returns an empty composite schema for plugins that doesn't
// take any payload.
func (PluginEnvironment) PayloadSchema() runtime.CompositeSchema {
	return runtime.NewEmptyCompositeSchema()
}

// NewPlugin returns nil which will be ignored
func (PluginEnvironment) NewPlugin(PluginOptions) (Plugin, error) {
	return nil, nil
}

// PluginBase is a base implementation of the plugin interface, it just handles
// all methods and does nothing.
//
// Implementors should embed this to ensure forward compatibility when we add
// new optional methods.
type PluginBase struct{}

// LogDrain ignores the log setup and returns nil
func (PluginBase) LogDrain() (io.Writer, error) {
	return nil, nil
}

// Prepare ignores the sandbox preparation stage.
func (PluginBase) Prepare(runtime.TaskContext) error {
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
