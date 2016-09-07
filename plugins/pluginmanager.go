package plugins

import (
	"errors"
	"fmt"
	"sync"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
)

type pluginManager struct {
	payloadSchema schematypes.Object
	plugins       []Plugin
}

type taskPluginManager struct {
	taskPlugins []TaskPlugin
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

// PluginManagerConfigSchema returns configuration for PluginOptions.Config for
// NewPluginManager.
func PluginManagerConfigSchema() schematypes.Object {
	plugins := Plugins()

	pluginNames := []string{}
	for name := range plugins {
		pluginNames = append(pluginNames, name)
	}

	s := schematypes.Object{
		MetaData: schematypes.MetaData{
			Title: "Plugin Configuration",
			Description: `Mapping from plugin name to plugin configuration.
                    The 'disabled' key is special and lists plugins that are
                    disabled. Plugins that are disabled do not require
                    configuration.`,
		},
		Properties: schematypes.Properties{
			"disabled": schematypes.Array{
				MetaData: schematypes.MetaData{
					Title: "Disabled Plugins",
					Description: `List of disabled plugins. If a plugin is not listed
												as disabled here, then its configuration key must be
												specified, unless it doesn't take any configuration.`,
				},
				Items: schematypes.StringEnum{
					Options: pluginNames,
				},
			},
		},
		Required: []string{"disabled"},
	}
	for name, provider := range plugins {
		cs := provider.ConfigSchema()
		if cs != nil {
			s.Properties[name] = cs
		}
	}
	return s
}

// stringContains returns true if list contains element
func stringContains(list []string, element string) bool {
	for _, s := range list {
		if s == element {
			return true
		}
	}
	return false
}

// NewPluginManager loads all plugins not disabled in configuration and
// returns a Plugin implementation that wraps all of the plugins.
//
// This expects options.Config satisfying schema from
// PluginManagerConfigSchema().
func NewPluginManager(options PluginOptions) (Plugin, error) {
	pluginProviders := Plugins()

	// Construct config schema
	configSchema := PluginManagerConfigSchema()

	// Ensure the config is valid
	if err := configSchema.Validate(options.Config); err != nil {
		return nil, fmt.Errorf("Invalid config, error: %s", err)
	}
	config := options.Config.(map[string]interface{})

	// Find plugins to load
	var enabled []string
	var disabled []string
	if configSchema.Properties["disabled"].Map(config["disabled"], &disabled) != nil {
		panic("internal error -- shouldn't be possible")
	}

	// Find list of enabled plugins and ensure that config is present if required
	for name, plugin := range pluginProviders {
		// Skip disabled plugins
		if stringContains(disabled, name) {
			continue
		}
		// Check that configuration is given if required
		if plugin.ConfigSchema() != nil {
			if _, ok := config[name]; !ok {
				return nil, fmt.Errorf("Missing configuration for plugin: '%s'", name)
			}
		}
		// List plugin as enabled
		enabled = append(enabled, name)
	}

	// Initialize all the plugins
	wg := sync.WaitGroup{}
	plugins := make([]Plugin, len(enabled))
	errors := make([]error, len(enabled))
	wg.Add(len(enabled))
	for index, name := range enabled {
		go func(index int, name string) {
			plugins[index], errors[index] = pluginProviders[name].NewPlugin(PluginOptions{
				Environment: options.Environment,
				Engine:      options.Engine,
				Log:         options.Log.WithField("plugin", name),
				Config:      config[name],
			})
			wg.Done()
		}(index, name) // needed to capture values not variables
	}
	wg.Wait()

	// Return the first error, if any
	for _, e := range errors {
		if e != nil {
			return nil, e
		}
	}

	// Construct payload schema
	schemas := []schematypes.Object{}
	for _, plugin := range plugins {
		schemas = append(schemas, plugin.PayloadSchema())
	}
	schema, err := schematypes.Merge(schemas...)
	if err != nil {
		return nil, fmt.Errorf("Conflicting payload schema types, error: %s", err)
	}

	return &pluginManager{
		plugins:       plugins,
		payloadSchema: schema,
	}, nil
}

func (m *pluginManager) PayloadSchema() schematypes.Object {
	return m.payloadSchema
}

func (m *pluginManager) NewTaskPlugin(options TaskPluginOptions) (TaskPlugin, error) {
	// Input must be valid
	if m.payloadSchema.Validate(options.Payload) != nil {
		return nil, engines.ErrContractViolation
	}

	// Create a list of TaskPlugins and a mutex to guard access
	taskPlugins := []TaskPlugin{}
	mu := sync.Mutex{}

	errors := make(chan error)
	for _, p := range m.plugins {
		go func(p Plugin) {
			taskPlugin, err := p.NewTaskPlugin(TaskPluginOptions{
				TaskInfo: options.TaskInfo,
				Payload:  p.PayloadSchema().Filter(options.Payload),
			})
			if taskPlugin != nil {
				mu.Lock()
				taskPlugins = append(taskPlugins, taskPlugin)
				mu.Unlock()
			}
			errors <- err
		}(p)
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
		go func(p TaskPlugin) { errors <- p.Prepare(c) }(p)
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
		go func(p TaskPlugin) { errors <- p.BuildSandbox(b) }(p)
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
		go func(p TaskPlugin) { errors <- p.Started(s) }(p)
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
		go func(p TaskPlugin) {
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
		go func(p TaskPlugin) { errors <- p.Finished(s) }(p)
	}
	return waitForErrors(errors, len(m.taskPlugins))
}

func (m *taskPluginManager) Exception(r runtime.ExceptionReason) error {
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
		go func(p TaskPlugin) { errors <- p.Exception(r) }(p)
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
		go func(p TaskPlugin) { errors <- p.Dispose() }(p)
	}
	return waitForErrors(errors, len(m.taskPlugins))
}
