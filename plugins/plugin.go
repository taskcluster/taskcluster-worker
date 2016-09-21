package plugins

import (
	"github.com/Sirupsen/logrus"
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

// The TaskPluginOptions is a wrapper for the set of arguments given to
// NewTaskPlugin.
//
// We wrap the arguments in a single argument to maintain source compatibility
// when introducing additional arguments.
type TaskPluginOptions struct {
	TaskInfo *runtime.TaskInfo
	Payload  map[string]interface{}
	Log      *logrus.Entry
	// Note: This is passed by-value for efficiency (and to prohibit nil), if
	// adding any large fields please consider adding them as pointers.
	// Note: This is intended to be a simple argument wrapper, do not add methods
	// to this struct.
}

// Plugin is a plugin to the worker, for each task NewTaskPlugin is created.
// The Plugin instance is responsible for creating these objects and managing
// data shared between TaskPlugin instances.
//
// All methods on this interface must be thread-safe.
type Plugin interface {
	// PayloadSchema returns a schematypes.Object with the properties for
	// for the TaskPluginOptions.Payload property.
	//
	// Note: this will be merged with payload schemas from engine and other
	// plugins, thus, it cannot contain conflicting properties. Furthermore the
	// metadata will be discarded and additionalProperties will not be allowed.
	PayloadSchema() schematypes.Object

	// NewTaskPlugin method will be called once for each task. The TaskPlugin
	// instance returned will be called for each stage in the task execution.
	//
	// This is a poor place to do any processing, and not a great place to start
	// long-running operations as you don't have a place to write log messages.
	// Consider waiting until Prepare() is called with TaskContext that you can
	// write log messages to.
	//
	// Plugins implementing logging should not return an error here, as it
	// naturally follows that such an error can't be logged if no logging plugin
	// is created.
	//
	// Implementors may return nil, if the plugin doesn't have any hooks for the
	// given tasks.
	//
	// Non-fatal errors: MalformedPayloadError
	NewTaskPlugin(options TaskPluginOptions) (TaskPlugin, error)
}

// TaskPlugin holds the task-specific state for a plugin
//
// Each method on this interface represents stage in the task execution and will
// be called when this stage is reached. The methods are allowed to
// take significant amounts of time, as they will run asynchronously.
//
// These methods does not have to be thread-safe, we will never call the next
// method, before the previous method has returned.
//
// Implementors of this interface should be sure to embed TaskPluginBase.
// This will do absolutely nothing, but provide empty implementations for any
// current and future methods that isn't implemented.
//
// The methods are called in the order listed here with the exception of
// Exception() which may be called following any method, and Dispose() which
// will always be called as a final step allowing you to clean up.
type TaskPlugin interface {
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
	BuildSandbox(SandboxBuilder engines.SandboxBuilder) error

	// Started is called once the sandbox has started execution. This is a good
	// place to hook if you want to do interactive things.
	//
	// Non-fatal errors: MalformedPayloadError
	Started(sandbox engines.Sandbox) error

	// Stopped is called once the sandbox has terminated.
	//
	// This is a good place to upload artifacts, logs, check exit code, and start
	// to clean-up resources if such clean-up is expected to take a while.
	//
	// This will return false if the operation could not be completed successfully.
	// Such as artifact upload failure or files not existing.  If this returns false
	// it shall be assumed that the task has failed and should be reported as a failure.
	//
	// Non-fatal errors: MalformedPayloadError
	Stopped(result engines.ResultSet) (bool, error)

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
	// Implementors should be aware that additional reasons may be added in the
	// future. Therefore they must handle the default case, if switching on the
	// reason parameter.
	Exception(reason runtime.ExceptionReason) error

	// Dispose is called once everything is done and it's time for clean-up.
	//
	// This method will be invoked following Stopped() or Exception(). It is then
	// the responsibility of the implementor to abort or wait for any long-running
	// processes and clean-up any resources held.
	Dispose() error
}

// PluginBase is a base implementation of the Plugin interface, it just
// handles all methods and does nothing.
//
// Implementors should embed this to ensure forward compatibility when we add
// new optional methods.
type PluginBase struct{}

// PayloadSchema returns a schema for an empty object for plugins that doesn't
// take any payload.
func (PluginBase) PayloadSchema() schematypes.Object {
	return schematypes.Object{}
}

// NewTaskPlugin returns nil ignoring the request to create a TaskPlugin for
// the given task.
func (PluginBase) NewTaskPlugin(TaskPluginOptions) (TaskPlugin, error) {
	return nil, nil
}

// TaskPluginBase is a base implementation of the TaskPlugin interface, it just
// handles all methods and does nothing.
//
// Implementors should embed this to ensure forward compatibility when we add
// new optional methods.
type TaskPluginBase struct{}

// Prepare ignores the sandbox preparation stage.
func (TaskPluginBase) Prepare(*runtime.TaskContext) error {
	return nil
}

// BuildSandbox ignores the sandbox building stage.
func (TaskPluginBase) BuildSandbox(engines.SandboxBuilder) error {
	return nil
}

// Started ignores the stage where the sandbox has started
func (TaskPluginBase) Started(engines.Sandbox) error {
	return nil
}

// Stopped ignores the stage where the sandbox has returned a ResultSet, and
// returns true saying the task was successful, as not to poison the water.
func (TaskPluginBase) Stopped(engines.ResultSet) (bool, error) {
	return true, nil
}

// Finished ignores the stage where a task has been finished
func (TaskPluginBase) Finished(success bool) error {
	return nil
}

// Exception ignores the stage where a task is resolved exception
func (TaskPluginBase) Exception(reason runtime.ExceptionReason) error {
	return nil
}

// Dispose ignores the stage where resources are disposed.
func (TaskPluginBase) Dispose() error {
	return nil
}
