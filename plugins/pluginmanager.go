package plugins

import (
	"sync"
	"sync/atomic"

	"github.com/taskcluster/taskcluster-worker/engine"
	rt "github.com/taskcluster/taskcluster-worker/runtime"
)

//TODO Find a home for AtomicBool
type atomicBool struct {
	value int
}

func (b *atomicBool) Set(value bool) {
	if value {
		return atomic.StoreInt32(&b.value, 1)
	}
	atomic.StoreInt32(&b.value, 0)
}
func (b *atomicBool) Swap(value bool) bool {
	if value {
		return atomic.SwapInt32(&b.value, 1) != 0
	}
	return atomic.SwapInt32(&b.value, 0) != 0
}
func (b *atomicBool) Get() bool {
	return atomic.LoadInt32(&b.value) != 0
}

// The pluginManager wraps multiple plugin providers
type pluginManager struct {
	providers []PluginProvider
}

type pluginWrapper struct {
	plugins []Plugin
	working atomicBool
}

// NewPluginProvider creates a PluginProvider that wraps all the other
// PluginProviders. This is also the place to register your plugin provider.
//
// Really there is no need to implement the PluginProvider interface, it's just
// elegant and if we make generic Plugin tests we can run them for the
// pluginManager too.
func NewPluginProvider(engine engine.Engine, ctx *rt.EngineContext) PluginProvider {
	return &pluginManager{
		providers: []PluginProvider{
		// List your plugins here
		// Example: plugins.mock.NewPluginProvider(engine, ctx)
		},
	}
}

func (m *pluginManager) NewPlugin(builder *rt.SandboxContextBuilder) Plugin {
	// Create an array for plugins
	plugins := make([]Plugin, len(m.providers))
	// Wait group, so we can wait for all plugins to finish
	var wg sync.WaitGroup
	wg.Add(len(m.providers))

	// For each provider we call NewPlugin() in parallel, just in case someone
	// doesn't something expensive.
	for i, provider := range m.providers {
		// I can't believe golang made the same mistake as javascript, so stupid
		// and it just recently got fixed in ES6
		i, provider := i, provider
		go func() {
			plugins[i] = provider.NewPlugin(builder)
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

func (w *pluginWrapper) Prepared(preparedSandbox engine.PreparedSandbox) error {
	// Sanity check that no two methods on plugin is running in parallel
	if w.working.Swap(true) {
		panic("Another plugin method is currently running, or Dispose() has been called!")
	}
	defer w.worker.Set(false)

	// Run method on plugins in parallel
	errors := make(chan error)
	for _, plugin := range w.plugins {
		plugin := plugin
		go func() { errors <- plugin.Prepared(preparedSandbox) }()
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
	var result atomicBool
	result.Set(true)

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
