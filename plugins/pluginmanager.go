package plugins

import (
	"sync"

	"github.com/taskcluster/taskcluster-worker/engine"
	"github.com/taskcluster/taskcluster-worker/plugins/success"
	rt "github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/utils"
)

// The pluginManager wraps multiple plugin factories
type pluginManager struct {
	factories []PluginFactory
}

type pluginWrapper struct {
	plugins []Plugin
	working utils.AtomicBool
}

// NewPluginFactory creates a PluginFactory that wraps all the other
// plugin factories. This is also the place to register your PluginFactory.
//
// Really there is no need to implement the PluginFactory interface, it's just
// elegant and if we make generic Plugin tests we can run them for the
// pluginManager too.
func NewPluginFactory(engine engine.Engine, ctx *rt.EngineContext) PluginFactory {
	return &pluginManager{
		// List your plugins here
		factories: []PluginFactory{
			// Success plugin ensures task is declared "failed" if execution isn't
			// success (exit code non-zero in most cases)
			success.NewPluginFactory(engine, ctx),
			// Dummy plugin that does absolutely nothing always added to make sure
			// that it doesn't have any side-effects
			PluginBase{},
		},
	}
}

func (m *pluginManager) NewPlugin(builder *rt.SandboxContextBuilder) Plugin {
	// Create an array for plugins
	plugins := make([]Plugin, len(m.factories))
	// Wait group, so we can wait for all plugins to finish
	var wg sync.WaitGroup
	wg.Add(len(m.factories))

	// For each factory we call NewPlugin() in parallel, just in case someone
	// doesn't something expensive.
	for i, factory := range m.factories {
		// I can't believe golang made the same mistake as javascript, so stupid
		// and it just recently got fixed in ES6
		i, factory := i, factory
		go func() {
			plugins[i] = factory.NewPlugin(builder)
			wg.Done()
		}()
	}

	// Wait for plugins to be create and return a plugin wrapper
	wg.Wait()
	return &pluginWrapper{
		plugins: plugins,
	}
}

// Plugin implementation for pluginWrapper

// waitForErrors returns it has received count results from the errors channel.
// Note, that errors might be nil, if all are nil it'll return nil otherwise
// it'll merge the errors.
func waitForErrors(errors <-chan error, count int) error {
	retval := nil
	for err := range errors {
		if err != nil {
			// TODO Merge the errors instead of taking the last
			// TODO MalformedPayloadError needs special care, if we only have these
			//      it is very different from having a list of other errors, Hence,
			//      we need to maintain the type rather than mindlessly wrap them.
			retval = err
		}
		count--
		if count == 0 {
			break
		}
	}
	return retval
}

func (w *pluginWrapper) Prepare(context *rt.SandboxContext) error {
	// Sanity check that no two methods on plugin is running in parallel, this way
	// plugins don't have to be thread-safe, and we ensure nothing is called after
	// Dispose() has been called.
	if w.working.Swap(true) {
		panic("Another plugin method is currently running, or Dispose() has been called!")
	}
	defer w.worker.Set(false)

	// Run method on plugins in parallel
	errors := make(chan error)
	for _, plugin := range w.plugins {
		plugin := plugin
		go func() { errors <- plugin.Prepare(context) }()
	}
	return waitForErrors(errors, len(m.plugins))
}

func (w *pluginWrapper) BuildSandbox(sandboxBuilder engine.SandboxBuilder) error {
	// Sanity check that no two methods on plugin is running in parallel
	if w.working.Swap(true) {
		panic("Another plugin method is currently running, or Dispose() has been called!")
	}
	defer w.worker.Set(false)

	// Run method on plugins in parallel
	errors := make(chan error)
	for _, plugin := range w.plugins {
		plugin := plugin
		go func() { errors <- plugin.BuildSandbox(sandboxBuilder) }()
	}
	return waitForErrors(errors, len(m.plugins))
}

func (w *pluginWrapper) Started(sandbox engine.Sandbox) error {
	// Sanity check that no two methods on plugin is running in parallel
	if w.working.Swap(true) {
		panic("Another plugin method is currently running, or Dispose() has been called!")
	}
	defer w.worker.Set(false)

	// Run method on plugins in parallel
	errors := make(chan error)
	for _, plugin := range w.plugins {
		plugin := plugin
		go func() { errors <- plugin.Started(sandbox) }()
	}
	return waitForErrors(errors, len(m.plugins))
}

func (w *pluginWrapper) Stopped(resultSet engine.ResultSet) (bool, error) {
	// Sanity check that no two methods on plugin is running in parallel
	if w.working.Swap(true) {
		panic("Another plugin method is currently running, or Dispose() has been called!")
	}
	defer w.working.Set(false)

	// Use atomic bool to return true, if no plugin returns false
	result := utils.NewAtomicBool(true)

	// Run method on plugins in parallel
	errors := make(chan error)
	for _, plugin := range w.plugins {
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
	err := waitForErrors(errors, len(m.plugins))
	// Return true if result was
	return result.Get(), err
}

func (w *pluginWrapper) Dispose() error {
	// Sanity check that no two methods on plugin is running in parallel
	if w.working.Swap(true) {
		panic("Another plugin method is currently running, or Dispose() has been called!")
	}
	// Notice that we don't call: defer w.working.Set(false), as we don't want to
	// allow any calls to plugins after Dispose()

	// Call dispose on all plugins in parallel
	errors := make(chan error)
	for _, plugin := range w.plugins {
		plugin := plugin
		go func() { errors <- plugin.Dispose() }()
	}
	return waitForErrors(errors, len(m.plugins))
}
