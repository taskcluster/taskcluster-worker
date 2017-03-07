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
	schematypes.MustValidate(configSchema, options.Config)
	config := options.Config.(map[string]interface{})

	// Find plugins to load
	var enabled []string
	var disabled []string
	schematypes.MustValidateAndMap(configSchema.Properties["disabled"], config["disabled"], &disabled)

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
	for i, j := range enabled {
		go func(index int, name string) {
			plugins[index], errors[index] = pluginProviders[name].NewPlugin(PluginOptions{
				Environment: options.Environment,
				Engine:      options.Engine,
				Monitor:     options.Monitor.WithPrefix(name).WithTag("plugin", name),
				Config:      config[name],
			})
			wg.Done()
		}(i, j) // needed to capture values not variables
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
	schematypes.MustValidate(m.payloadSchema, options.Payload)

	taskPlugins := make([]TaskPlugin, len(m.plugins))
	errors := make([]error, len(m.plugins))

	var wg sync.WaitGroup
	for i, j := range m.plugins {
		wg.Add(1)
		go func(index int, p Plugin) {
			defer wg.Done()
			taskPlugins[index], errors[index] = p.NewTaskPlugin(TaskPluginOptions{
				TaskInfo:    options.TaskInfo,
				TaskContext: options.TaskContext,
				Payload:     p.PayloadSchema().Filter(options.Payload),
				Monitor:     options.Monitor.WithPrefix(m.pluginNames[index]).WithTag("plugin", m.pluginNames[index]),
			})
		}(i, j)
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

type taskPluginPhase func(TaskPlugin) error

func (m *taskPluginManager) executePhase(f taskPluginPhase) error {
	// Sanity check that no two methods on plugin is running in parallel, this way
	// plugins don't have to be thread-safe, and we ensure nothing is called after
	// Dispose() has been called.
	if m.working.Swap(true) {
		panic("Another plugin method is currently running, or Dispose() has been called!")
	}
	defer m.working.Set(false)

	// Execute phase on plugins in parallel
	errors := make([]error, len(m.taskPlugins))
	var wg sync.WaitGroup
	for i, j := range m.taskPlugins {
		wg.Add(1)
		go func(index int, tp TaskPlugin) {
			defer wg.Done()
			errors[index] = f(tp)
		}(i, j)
	}
	wg.Wait()
	// Returned error represents the merge of all errors which occurred against
	// any plugin, or nil if no error occurred.
	return mergeErrors(errors...)
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
