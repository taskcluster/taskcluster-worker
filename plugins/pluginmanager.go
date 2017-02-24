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
	pluginNames   []string
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

// waitForErrors executes function f against every plugin passed in taskPlugins
// and returns an error which represents the merge of all errors which occurred
// against any plugin.
//
// Note, that errors might be nil, if all are nil it'll return nil otherwise
// it'll merge the errors.
func waitForErrors(taskPlugins []TaskPlugin, f func(p TaskPlugin) error) (err error) {
	errors := make([]error, len(taskPlugins))
	var wg sync.WaitGroup
	for i, j := range taskPlugins {
		wg.Add(1)
		go func(i int, j TaskPlugin) {
			defer wg.Done()
			errors[i] = f(j)
		}(i, j)
	}
	wg.Wait()
	return mergeErrors(errors...)
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
		pluginNames:   enabled,
		payloadSchema: schema,
	}, nil
}

func (m *pluginManager) PayloadSchema() schematypes.Object {
	return m.payloadSchema
}

func (m *pluginManager) NewTaskPlugin(options TaskPluginOptions) (manager TaskPlugin, err error) {
	// Input must be valid
	if m.payloadSchema.Validate(options.Payload) != nil {
		return nil, engines.ErrContractViolation
	}

	taskPlugins := make([]TaskPlugin, len(m.plugins))
	errors := make([]error, len(m.plugins))

	var wg sync.WaitGroup
	for i, p := range m.plugins {
		wg.Add(1)
		go func(i int, p Plugin) {
			defer wg.Done()
			taskPlugins[i], errors[i] = p.NewTaskPlugin(TaskPluginOptions{
				TaskInfo: options.TaskInfo,
				Payload:  p.PayloadSchema().Filter(options.Payload),
				Log:      options.Log.WithField("plugin", m.pluginNames[i]),
			})
		}(i, p)
	}
	wg.Wait()
	err = mergeErrors(errors...)

	manager = &taskPluginManager{
		taskPlugins: taskPlugins,
	}

	if err != nil {
		err2 := manager.Dispose()
		return nil, mergeErrors(err, err2)
	}
	return
}

func (m *taskPluginManager) executePhase(f func(p TaskPlugin) error) error {
	// Sanity check that no two methods on plugin is running in parallel, this way
	// plugins don't have to be thread-safe, and we ensure nothing is called after
	// Dispose() has been called.
	if m.working.Swap(true) {
		panic("Another plugin method is currently running, or Dispose() has been called!")
	}
	defer m.working.Set(false)

	// Run method on plugins in parallel
	return waitForErrors(m.taskPlugins, f)
}

func (m *taskPluginManager) Prepare(c *runtime.TaskContext) error {
	return m.executePhase(func(p TaskPlugin) error { return p.Prepare(c) })
}

func (m *taskPluginManager) BuildSandbox(b engines.SandboxBuilder) error {
	return m.executePhase(func(p TaskPlugin) error { return p.BuildSandbox(b) })
}

func (m *taskPluginManager) Started(s engines.Sandbox) error {
	return m.executePhase(func(p TaskPlugin) error { return p.Started(s) })
}

func (m *taskPluginManager) Stopped(r engines.ResultSet) (bool, error) {
	// Use atomic bool to return true, if no plugin returns false
	result := atomics.NewBool(true)

	// Run method on plugins in parallel
	err := m.executePhase(func(p TaskPlugin) error {
		success, err := p.Stopped(r)
		if !success {
			result.Set(false)
		}
		return err
	})
	return result.Get(), err
}

func (m *taskPluginManager) Finished(s bool) error {
	return m.executePhase(func(p TaskPlugin) error { return p.Finished(s) })
}

func (m *taskPluginManager) Exception(r runtime.ExceptionReason) error {
	return m.executePhase(func(p TaskPlugin) error { return p.Exception(r) })
}

func (m *taskPluginManager) Dispose() error {
	// we don't want to allow any calls to plugins after Dispose()
	defer m.working.Set(true)
	return m.executePhase(func(p TaskPlugin) error { return p.Dispose() })
}
