package plugins

import (
	"time"

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
	TaskInfo    *runtime.TaskInfo
	TaskContext *runtime.TaskContext
	Payload     map[string]interface{}
	Monitor     runtime.Monitor
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
	// Documentation returns a list of sections with end-user documentation.
	//
	// These sections will be combined with documentation sections from all
	// enabled plugins in-order to form end-user documentation.
	Documentation() []runtime.Section

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
	// NewTaskPlugin will be called in parallel with NewSandboxBuilder(), making
	// it a good place to start long-running operations, you then have to take
	// care to clean-up in Dispose() if they are still running.
	// You should wait for your long-running operations to finish in
	//  BuildSandbox() or whatever hook you need them in.
	//
	// Plugins implementing logging should not return an error here, as it
	// naturally follows that such an error can't be logged if no logging plugin
	// is created.
	//
	// Implementors may return TaskPluginBase{}, if the plugin doesn't have any
	// hooks for the given tasks.
	//
	// Non-fatal errors: MalformedPayloadError
	NewTaskPlugin(options TaskPluginOptions) (TaskPlugin, error)

	// ReportIdle is called if the worker is idle prior to polling for tasks.
	// The durationSinceBusy is the time since the worker as last busy.
	//
	// This is a perfect time to initiate a graceful stop the worker by
	// calling runtime.Environment.Worker.StopGracefully(). A plugin can call
	// StopGracefully() at any time, causing the worker to stop polling.
	// This hook is mainly useful as a way to gracefully stop the worker without
	// entering a new billing cycle, which might happen if you gracefully stop
	// while the worker is processing one or more tasks.
	//
	// Warning: This hook is called as part of the worker claim loop, hence, it
	// strongly to execution time in this method
	ReportIdle(durationSinceBusy time.Duration)

	// ReportNonFatalError is called if the worker encountered a non-fatal error.
	//
	// Plugins can use this to implement a heuristic for shutting down after
	// certain number of non-fatal errors, or purging caches. This is mainly
	// intended to allow a configurable plugin to decide if the worker should
	// stop in response to a non-fatal error.
	//
	// The specific non-fatal error message is not available to the heuristic.
	// If a special heuristic is desired for a special non-fatal error, then this
	// should be handled in the plugin/engine where the error origins.
	ReportNonFatalError()

	// Dispose is called when the worker is stopping, a plugin should free all
	// resources and halt all background processes.
	//
	// This is important for testing, but also for relevant when running on
	// bare metal machines that aren't re-imaged between worker restarts.
	Dispose() error
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
	// BuildSandbox is called once NewSandboxBuilder() has returned.
	//
	// This is the place to wait for downloads and other expensive operations to
	// finished, before mounting caches, proxies, etc. and returning.
	//
	// Non-fatal errors: MalformedPayloadError
	BuildSandbox(sandboxBuilder engines.SandboxBuilder) error

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
	// This method will be invoked following Finished() or Exception(). It is then
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

// Documentation returns no documentation.
func (PluginBase) Documentation() []runtime.Section {
	return nil
}

// PayloadSchema returns a schema for an empty object for plugins that doesn't
// take any payload.
func (PluginBase) PayloadSchema() schematypes.Object {
	return schematypes.Object{}
}

// NewTaskPlugin returns TaskPluginBase{} which ignores all the stages.
func (PluginBase) NewTaskPlugin(TaskPluginOptions) (TaskPlugin, error) {
	return TaskPluginBase{}, nil
}

// ReportIdle does nothing
func (PluginBase) ReportIdle(time.Duration) {}

// ReportNonFatalError does nothing
func (PluginBase) ReportNonFatalError() {}

// Dispose does nothing
func (PluginBase) Dispose() error {
	return nil
}

// TaskPluginBase is a base implementation of the TaskPlugin interface, it just
// handles all methods and does nothing.
//
// Implementors should embed this to ensure forward compatibility when we add
// new optional methods.
type TaskPluginBase struct{}

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
