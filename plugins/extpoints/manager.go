package extpoints

import (
	"errors"
	"sync"

	"github.com/taskcluster/taskcluster-worker/atomics"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type pluginManager struct {
	plugins []plugins.Plugin
}

type taskPluginManager struct {
	taskPlugins []plugins.TaskPlugin
	working     atomics.Bool
}

// mergeErrors merges a list of errors into one error, ignoring nil entries,
// and ensuring that if there is only MalformedPayloadErrors these will be
// merged into one MalformedPayloadError
func mergeErrors(errs ...error) error {
	msg := ""
	for _, err := range errs {
		if err != nil {
			// TODO MalformedPayloadError needs special care, if we only have these
			//      it is very different from having a list of other errors, Hence,
			//      we need to maintain the type rather than mindlessly wrap them.
			msg += err.Error() + "\n"
		}
	}
	if len(msg) > 0 {
		return errors.New(msg)
	}
	return nil
}

// waitForErrors returns when it has received count results from the errors
// channel.
//
// Note, that errors might be nil, if all are nil it'll return nil otherwise
// it'll merge the errors.
func waitForErrors(errors <-chan error, count int) error {
	errs := []error{}
	for err := range errors {
		if err != nil {
			errs = append(errs, err)
		}
		count--
		if count == 0 {
			break
		}
	}
	return mergeErrors(errs...)
}

// NewPluginManager loads the list of plugins it is given and returns a single
// Plugin implementation that wraps all of the plugins.
func NewPluginManager(pluginsToLoad []string, options PluginOptions) (plugins.Plugin, error) {
	// Initialize all the requested plugins
	pluginObjects := []plugins.Plugin{}
	for _, p := range pluginsToLoad {
		//TODO: Do this plugin initialization in parallel for better performance
		pluginProvider := PluginProviders.Lookup(p)
		if pluginProvider == nil {
			return nil, errors.New("Missing plugin")
		}
		plugin, err := pluginProvider(PluginOptions{
			environment: options.environment,
			engine:      options.engine,
			log:         options.log.WithField("plugin", p),
		})
		if err != nil {
			return nil, err
		}
		pluginObjects = append(pluginObjects, plugin)
	}
	return &pluginManager{plugins: pluginObjects}, nil
}

func (m *pluginManager) PayloadSchema() (runtime.CompositeSchema, error) {
	schemas := []runtime.CompositeSchema{}
	for _, plugin := range m.plugins {
		schema, err := plugin.PayloadSchema()
		if err != nil {
			return nil, err
		}
		schemas = append(schemas, schema)
	}
	return runtime.MergeCompositeSchemas(schemas...)
}

func (m *pluginManager) NewTaskPlugin(options plugins.TaskPluginOptions) (plugins.TaskPlugin, error) {
	// Since this plugin uses MergeCompositeSchemas to return a CompositeSchema
	// we know from MergeCompositeSchemas that the Payload must be an array
	payload := options.Payload.([]interface{})

	// Create a list of TaskPlugins and a mutex to guard access
	taskPlugins := []plugins.TaskPlugin{}
	mu := sync.Mutex{}

	errors := make(chan error)
	for i, p := range m.plugins {
		go func(i int, p plugins.Plugin) {
			taskPlugin, err := p.NewTaskPlugin(plugins.TaskPluginOptions{
				TaskInfo: options.TaskInfo,
				Payload:  payload[i],
			})
			if taskPlugin != nil {
				mu.Lock()
				taskPlugins = append(taskPlugins, taskPlugin)
				mu.Unlock()
			}
			errors <- err
		}(i, p)
	}

	// Wait for errors, if any we dispose and return a merged error
	err := waitForErrors(errors, len(m.plugins))
	manager := &taskPluginManager{taskPlugins: taskPlugins}
	if err != nil {
		err2 := manager.Dispose()
		return nil, mergeErrors(err, err2)
	}
	return manager, nil
}

func (m *taskPluginManager) Prepare(c *runtime.TaskContext) error {
	// Sanity check that no two methods on plugin is running in parallel, this way
	// plugins don't have to be thread-safe, and we ensure nothing is called after
	// Dispose() has been called.
	if m.working.Swap(true) {
		panic("Another plugin method is currently running, or Dispose() has been called!")
	}
	defer m.working.Set(false)

	// Run method on plugins in parallel
	errors := make(chan error)
	for _, p := range m.taskPlugins {
		go func(p plugins.TaskPlugin) { errors <- p.Prepare(c) }(p)
	}
	return waitForErrors(errors, len(m.taskPlugins))
}

func (m *taskPluginManager) BuildSandbox(b engines.SandboxBuilder) error {
	// Sanity check that no two methods on plugin is running in parallel, this way
	// plugins don't have to be thread-safe, and we ensure nothing is called after
	// Dispose() has been called.
	if m.working.Swap(true) {
		panic("Another plugin method is currently running, or Dispose() has been called!")
	}
	defer m.working.Set(false)

	// Run method on plugins in parallel
	errors := make(chan error)
	for _, p := range m.taskPlugins {
		go func(p plugins.TaskPlugin) { errors <- p.BuildSandbox(b) }(p)
	}
	return waitForErrors(errors, len(m.taskPlugins))
}

func (m *taskPluginManager) Started(s engines.Sandbox) error {
	// Sanity check that no two methods on plugin is running in parallel, this way
	// plugins don't have to be thread-safe, and we ensure nothing is called after
	// Dispose() has been called.
	if m.working.Swap(true) {
		panic("Another plugin method is currently running, or Dispose() has been called!")
	}
	defer m.working.Set(false)

	// Run method on plugins in parallel
	errors := make(chan error)
	for _, p := range m.taskPlugins {
		go func(p plugins.TaskPlugin) { errors <- p.Started(s) }(p)
	}
	return waitForErrors(errors, len(m.taskPlugins))
}

func (m *taskPluginManager) Stopped(r engines.ResultSet) (bool, error) {
	// Sanity check that no two methods on plugin is running in parallel, this way
	// plugins don't have to be thread-safe, and we ensure nothing is called after
	// Dispose() has been called.
	if m.working.Swap(true) {
		panic("Another plugin method is currently running, or Dispose() has been called!")
	}
	defer m.working.Set(false)

	// Use atomic bool to return true, if no plugin returns false
	result := atomics.NewBool(true)

	// Run method on plugins in parallel
	errors := make(chan error)
	for _, p := range m.taskPlugins {
		go func(p plugins.TaskPlugin) {
			success, err := p.Stopped(r)
			if !success {
				result.Set(false)
			}
			errors <- err
		}(p)
	}
	return result.Get(), waitForErrors(errors, len(m.taskPlugins))
}

func (m *taskPluginManager) Finished(s bool) error {
	// Sanity check that no two methods on plugin is running in parallel, this way
	// plugins don't have to be thread-safe, and we ensure nothing is called after
	// Dispose() has been called.
	if m.working.Swap(true) {
		panic("Another plugin method is currently running, or Dispose() has been called!")
	}
	defer m.working.Set(false)

	// Run method on plugins in parallel
	errors := make(chan error)
	for _, p := range m.taskPlugins {
		go func(p plugins.TaskPlugin) { errors <- p.Finished(s) }(p)
	}
	return waitForErrors(errors, len(m.taskPlugins))
}

func (m *taskPluginManager) Exception(r plugins.ExceptionReason) error {
	// Sanity check that no two methods on plugin is running in parallel, this way
	// plugins don't have to be thread-safe, and we ensure nothing is called after
	// Dispose() has been called.
	if m.working.Swap(true) {
		panic("Another plugin method is currently running, or Dispose() has been called!")
	}
	defer m.working.Set(false)

	// Run method on plugins in parallel
	errors := make(chan error)
	for _, p := range m.taskPlugins {
		go func(p plugins.TaskPlugin) { errors <- p.Exception(r) }(p)
	}
	return waitForErrors(errors, len(m.taskPlugins))
}

func (m *taskPluginManager) Dispose() error {
	// Sanity check that no two methods on plugin is running in parallel
	if m.working.Swap(true) {
		panic("Another plugin method is currently running, or Dispose() has been called!")
	}
	// Notice that we don't call: defer w.working.Set(false), as we don't want to
	// allow any calls to plugins after Dispose()

	// Run method on plugins in parallel
	errors := make(chan error)
	for _, p := range m.taskPlugins {
		go func(p plugins.TaskPlugin) { errors <- p.Dispose() }(p)
	}
	return waitForErrors(errors, len(m.taskPlugins))
}
