package plugins

import (
	"github.com/taskcluster/taskcluster-worker/engines"
	rt "github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/utils"
)

// The PluginOptions is a wrapper for the set of arguments given to NewPlugin.
//
// We wrap the arguments in a single argument to maintain source compatibility
// when introducing additional arguments.
type PluginOptions struct {
	Task *rt.TaskContext
}

// Plugin Manager is responsible for maintaining a set of plugins initialized
// for a given task execution.
//
// Wrappers around the stages of the plugin lifecycle are given to ensure all
// plugins complete their stage before moving on.
type PluginManager struct {
	Plugins []Plugin
	working utils.AtomicBool
}

// Plugin implementation for PluginManager

// waitForErrors returns it has received count results from the errors channel.
// Note, that errors might be nil, if all are nil it'll return nil otherwise
// it'll merge the errors.
func waitForErrors(errors <-chan error, count int) []error {
	var retval []error
	for err := range errors {
		if err != nil {
			// TODO MalformedPayloadError needs special care, if we only have these
			//      it is very different from having a list of other errors, Hence,
			//      we need to maintain the type rather than mindlessly wrap them.
			retval = append(retval, err)
		}
		count--
		if count == 0 {
			break
		}
	}
	return retval
}

func (w *PluginManager) Prepare(context rt.TaskContext) []error {
	// Sanity check that no two methods on plugin is running in parallel, this way
	// plugins don't have to be thread-safe, and we ensure nothing is called after
	// Dispose() has been called.
	if w.working.Swap(true) {
		panic("Another plugin method is currently running, or Dispose() has been called!")
	}
	defer w.working.Set(false)

	// Run method on plugins in parallel
	errors := make(chan error)
	for _, plugin := range w.Plugins {
		plugin := plugin
		go func() { errors <- plugin.Prepare(context) }()
	}
	return waitForErrors(errors, len(w.Plugins))
}

func (w *PluginManager) BuildSandbox(sandboxBuilder engines.SandboxBuilder) []error {
	// Sanity check that no two methods on plugin is running in parallel
	if w.working.Swap(true) {
		panic("Another plugin method is currently running, or Dispose() has been called!")
	}
	defer w.working.Set(false)

	// Run method on plugins in parallel
	errors := make(chan error)
	for _, plugin := range w.Plugins {
		plugin := plugin
		go func() { errors <- plugin.BuildSandbox(sandboxBuilder) }()
	}
	return waitForErrors(errors, len(w.Plugins))
}

func (w *PluginManager) Started(sandbox engines.Sandbox) []error {
	// Sanity check that no two methods on plugin is running in parallel
	if w.working.Swap(true) {
		panic("Another plugin method is currently running, or Dispose() has been called!")
	}
	defer w.working.Set(false)

	// Run method on plugins in parallel
	errors := make(chan error)
	for _, plugin := range w.Plugins {
		plugin := plugin
		go func() { errors <- plugin.Started(sandbox) }()
	}
	return waitForErrors(errors, len(w.Plugins))
}

func (w *PluginManager) Stopped(resultSet engines.ResultSet) (bool, []error) {
	// Sanity check that no two methods on plugin is running in parallel
	if w.working.Swap(true) {
		panic("Another plugin method is currently running, or Dispose() has been called!")
	}
	defer w.working.Set(false)

	// Use atomic bool to return true, if no plugin returns false
	result := utils.NewAtomicBool(true)

	// Run method on plugins in parallel
	errors := make(chan error)
	for _, plugin := range w.Plugins {
		plugin := plugin
		go func() {
			success, err := plugin.Stopped(resultSet)
			if !success {
				result.Set(false)
			}
			errors <- err
		}()
	}
	// Wait for errors (before we read the result variable)
	errs := waitForErrors(errors, len(w.Plugins))
	// Return true if result was
	return result.Get(), errs
}

func (w *PluginManager) Dispose() []error {
	// Sanity check that no two methods on plugin is running in parallel
	if w.working.Swap(true) {
		panic("Another plugin method is currently running, or Dispose() has been called!")
	}
	// Notice that we don't call: defer w.working.Set(false), as we don't want to
	// allow any calls to plugins after Dispose()

	// Call dispose on all plugins in parallel
	errors := make(chan error)
	for _, plugin := range w.Plugins {
		plugin := plugin
		go func() { errors <- plugin.Dispose() }()
	}
	return waitForErrors(errors, len(w.Plugins))
}
