package plugins

import (
	"io"

	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

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
	PayloadSchema() runtime.CompositeSchema
	// Prepare will be called in parallel with NewSandboxBuilder().
	//
	// Notice that this method is a good place to start long-running operations,
	// you then have to take care to clean-up in Dispose() if they are still
	// running. You should wait for your long-running operations to finish in
	// BuildSandbox() or whatever hook you need them in.
	//
	// Non-fatal errors: MalformedPayloadError
	Prepare(context runtime.TaskContext) error

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

	// Stopped is called once the sandbox has terminated. Returns true, if the
	// task execution was successful.
	//
	// This is a good place to upload artifacts, logs, check exit code, and start
	// to clean-up resources if such clean-up is expected to take a while.
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

// PluginBase is a base implementation of the plugin interface, it just handles
// all methods and does nothing.
//
// Implementors should embed this to ensure forward compatibility when we add
// new optional methods.
type PluginBase struct{}

// PayloadSchema returns an empty composite schema for plugins that doesn't
// take any payload.
func (PluginBase) PayloadSchema() runtime.CompositeSchema {
	return runtime.NewEmptyCompositeSchema()
}

// LogDrain ignores the log setup and returns nil
func (PluginBase) LogDrain() (io.Writer, error) {
	return nil, nil
}

// Prepare ignores the sandbox preparation stage.
func (PluginBase) Prepare(runtime.TaskContext) error {
	return nil
}

// BuildSandbox ignores the sandbox building stage.
func (PluginBase) BuildSandbox(engines.SandboxBuilder) error {
	return nil
}

// Started ignores the stage where the sandbox has started
func (PluginBase) Started(engines.Sandbox) error {
	return nil
}

// Stopped ignores the stage where the sandbox has returned a ResultSet, and
// returns true saying the task was successful, as not to poison the water.
func (PluginBase) Stopped(engines.ResultSet) (bool, error) {
	return true, nil
}

// Dispose ignores the stage where resources are disposed.
func (PluginBase) Dispose() error {
	return nil
}

func (PluginBase) Exception(reason ExceptionReason) error {
	return nil
}

func (PluginBase) Finished(success bool) error {
	return nil
}
