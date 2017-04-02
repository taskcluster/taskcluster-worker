package plugins

import (
	"fmt"
	"sync"
	"time"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

type pluginManager struct {
	environment   runtime.Environment
	payloadSchema schematypes.Object
	monitor       runtime.Monitor
	plugins       []Plugin
	pluginNames   []string
	monitors      []runtime.Monitor
}

type taskPluginManager struct {
	monitor     runtime.Monitor
	taskPlugins []TaskPlugin
	monitors    []runtime.Monitor
	context     *runtime.TaskContext
	working     atomics.Bool
}

func spawn(n int, fn func(int)) {
	wg := sync.WaitGroup{}
	wg.Add(n)
	for index := 0; index < n; index++ {
		go func(i int) {
			defer wg.Done()
			fn(i)
		}(index)
	}
	wg.Wait()
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
										A plugin is enabled if it has an entry in this mapping, and
										isn't explicitly listed as 'disabled'. Even plugins that
										don't require configuration must have an entry, in these
										cases, empty object will suffice.`,
		},
		Properties: schematypes.Properties{
			"disabled": schematypes.Array{
				MetaData: schematypes.MetaData{
					Title: "Disabled Plugins",
					Description: `List of disabled plugins. If a plugin is not listed
												as disabled here, even if its configuration key is
												present`,
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
		} else {
			s.Properties[name] = schematypes.Object{}
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

	// Find list of enabled plugins
	for name := range config {
		// Ignore disabled plugins as well as the 'disabled' key
		if !stringContains(disabled, name) && name != "disabled" {
			enabled = append(enabled, name)
		}
	}

	// Initialize all the plugins
	plugins := make([]Plugin, len(enabled))
	errors := make([]error, len(enabled))
	monitors := make([]runtime.Monitor, len(enabled))
	spawn(len(enabled), func(i int) {
		name := enabled[i]
		monitors[i] = options.Monitor.WithPrefix(name).WithTag("plugin", name)
		incidentID := monitors[i].CapturePanic(func() {
			plugins[i], errors[i] = pluginProviders[name].NewPlugin(PluginOptions{
				Environment: options.Environment,
				Engine:      options.Engine,
				Monitor:     monitors[i],
				Config:      config[name],
			})
			if errors[i] != nil && plugins[i] == nil {
				panic(fmt.Sprintf("expected error or plugin from NewPlugin() from '%s'", name))
			}
		})
		if incidentID != "" {
			errors[i] = fmt.Errorf(
				"panic while calling NewPlugin for '%s' incidentId: %s",
				name, incidentID,
			)
		}
	})

	// Combine errors if any
	var msgs util.StringList
	for i, err := range errors {
		if err != nil {
			msgs.Sprintf("failed to instantiate plugin: '%s', error: %s", enabled[i], err)
		}
	}
	if len(msgs) > 0 {
		return nil, fmt.Errorf("plugin instantiation failed: - \n%s", msgs.Join("\n - "))
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
		environment:   *options.Environment,
		plugins:       plugins,
		pluginNames:   enabled,
		payloadSchema: schema,
		monitors:      monitors,
		monitor:       options.Monitor.WithPrefix("manager").WithTag("plugin", "manager"),
	}, nil
}

func (pm *pluginManager) ReportIdle(durationSinceBusy time.Duration) {
	spawn(len(pm.plugins), func(i int) {
		m := pm.monitors[i].WithTag("hook", "ReportIdle")
		incidentID := m.CapturePanic(func() {
			pm.plugins[i].ReportIdle(durationSinceBusy)
		})
		if incidentID != "" {
			m.Errorf("stopping worker now due to panic reported as incidentID=%s", incidentID)
			pm.environment.Worker.StopNow()
		}
	})
}

func (pm *pluginManager) ReportNonFatalError() {
	spawn(len(pm.plugins), func(i int) {
		m := pm.monitors[i].WithTag("hook", "ReportNonFatalError")
		incidentID := m.CapturePanic(func() {
			pm.plugins[i].ReportNonFatalError()
		})
		if incidentID != "" {
			m.Errorf("stopping worker now due to panic reported as incidentID=%s", incidentID)
			pm.environment.Worker.StopNow()
		}
	})
}

func (pm *pluginManager) Dispose() error {
	fatal := atomics.NewBool(false)
	nonfatal := atomics.NewBool(false)

	spawn(len(pm.plugins), func(i int) {
		m := pm.monitors[i].WithTag("hook", "Dispose")
		var err error
		incidentID := m.CapturePanic(func() {
			err = pm.plugins[i].Dispose()
		})
		switch err {
		case runtime.ErrFatalInternalError:
			fatal.Set(true)
		case runtime.ErrNonFatalInternalError:
			nonfatal.Set(true)
		case nil:
		default:
			incidentID = m.ReportError(err, "failed to dispose plugin")
		}
		if incidentID != "" {
			fatal.Set(true)
			m.Errorf("stopping worker now due to panic reported as incidentID=%s", incidentID)
			pm.environment.Worker.StopNow()
		}
	})

	if fatal.Get() {
		return runtime.ErrFatalInternalError
	}
	if nonfatal.Get() {
		return runtime.ErrNonFatalInternalError
	}
	return nil
}

func (pm *pluginManager) PayloadSchema() schematypes.Object {
	return pm.payloadSchema
}

func (pm *pluginManager) NewTaskPlugin(options TaskPluginOptions) (TaskPlugin, error) {
	// Input must be valid
	schematypes.MustValidate(pm.payloadSchema, options.Payload)

	N := len(pm.plugins)
	m := &taskPluginManager{
		monitor:     options.Monitor.WithPrefix("manager").WithTag("plugin", "manager"),
		taskPlugins: make([]TaskPlugin, N),
		monitors:    make([]runtime.Monitor, N),
		context:     options.TaskContext,
	}

	// Create monitors
	for i := 0; i < N; i++ {
		m.monitors[i] = options.Monitor.WithPrefix(pm.pluginNames[i]).WithTag("plugin", pm.pluginNames[i])
	}

	// Create taskPlugins
	err := m.spawnEachPlugin("NewTaskPlugin", func(i int) error {
		var nerr error
		m.taskPlugins[i], nerr = pm.plugins[i].NewTaskPlugin(TaskPluginOptions{
			TaskInfo:    options.TaskInfo,
			TaskContext: options.TaskContext,
			Payload:     pm.plugins[i].PayloadSchema().Filter(options.Payload),
			Monitor:     m.monitors[i],
		})
		return nerr
	})

	if err != nil {
		derr := m.Dispose()
		if derr == runtime.ErrFatalInternalError || err == runtime.ErrFatalInternalError {
			return nil, runtime.ErrFatalInternalError
		}
		if derr == runtime.ErrNonFatalInternalError || err == runtime.ErrNonFatalInternalError {
			return nil, runtime.ErrNonFatalInternalError
		}
		return nil, err
	}
	return m, nil
}

// spawnEachPlugin will invoke fn(i) for each plugin 0 to N. Any error or panic
// will be reported to sentry. MalformedPayloadErrors will be merged and returned,
// unless overruled by a ErrFatalInternalError or ErrNonFatalInternalError.
func (m *taskPluginManager) spawnEachPlugin(hook string, fn func(i int) error) error {
	N := len(m.taskPlugins)

	// Sanity check that no two methods on plugin is running in parallel, this way
	// plugins don't have to be thread-safe, and we ensure nothing is called after
	// Dispose() has been called.
	if m.working.Swap(true) {
		panic("Another plugin method is currently running, or Dispose() has been called!")
	}
	defer m.working.Set(false)

	errors := make([]error, N)
	spawn(N, func(i int) {
		monitor := m.monitors[i].WithTag("hook", hook)
		incidentID := monitor.CapturePanic(func() {
			errors[i] = fn(i)
		})
		if _, ok := runtime.IsMalformedPayloadError(errors[i]); !ok && errors[i] != nil {
			// Both of these errors assumes that the error has been logged and recorded
			if errors[i] != runtime.ErrFatalInternalError && errors[i] != runtime.ErrNonFatalInternalError {
				incidentID = monitor.ReportError(errors[i], "Unhandled error during ", hook, " hook")
			}
		}
		if incidentID != "" {
			errors[i] = runtime.ErrFatalInternalError
			m.context.LogError("Unhandled worker error encountered incidentID=", incidentID)
		}
	})

	// Find out if we have fatal errors, non-fatal errors and merge malformed
	// payload errors
	fatalErr := false
	nonFatalErr := false
	malformedErrs := []runtime.MalformedPayloadError{}
	for _, err := range errors {
		if err == runtime.ErrFatalInternalError {
			fatalErr = true
		}
		if err == runtime.ErrNonFatalInternalError {
			nonFatalErr = true
		}
		if e, ok := runtime.IsMalformedPayloadError(err); ok {
			malformedErrs = append(malformedErrs, e)
		}
	}

	var err error
	if nonFatalErr {
		err = runtime.ErrNonFatalInternalError
	}
	if fatalErr {
		err = runtime.ErrFatalInternalError
	}
	if len(malformedErrs) > 0 {
		if err == nil {
			err = runtime.MergeMalformedPayload(malformedErrs...)
		} else {
			m.context.LogError("Encountered an unhandled worker error, along with malformed payload errors")
			for _, e := range malformedErrs {
				m.context.LogError("MalformedPayloadError: ", e.Error())
			}
		}
	}
	return err
}

func (m *taskPluginManager) BuildSandbox(b engines.SandboxBuilder) error {
	return m.spawnEachPlugin("BuildSandbox", func(i int) error {
		return m.taskPlugins[i].BuildSandbox(b)
	})
}

func (m *taskPluginManager) Started(s engines.Sandbox) error {
	return m.spawnEachPlugin("Started", func(i int) error {
		return m.taskPlugins[i].Started(s)
	})
}

func (m *taskPluginManager) Stopped(r engines.ResultSet) (bool, error) {
	// Use atomic bool to return true, if no plugin returns false
	result := atomics.NewBool(true)

	// Run method on plugins in parallel
	err := m.spawnEachPlugin("Stopped", func(i int) error {
		success, perr := m.taskPlugins[i].Stopped(r)
		if !success {
			result.Set(false)
		}
		return perr
	})

	return result.Get(), err
}

func (m *taskPluginManager) Finished(s bool) error {
	return m.spawnEachPlugin("Finished", func(i int) error {
		return m.taskPlugins[i].Finished(s)
	})
}

func (m *taskPluginManager) Exception(r runtime.ExceptionReason) error {
	return m.spawnEachPlugin("Exception", func(i int) error {
		return m.taskPlugins[i].Exception(r)
	})
}

func (m *taskPluginManager) Dispose() error {
	// we don't want to allow any calls to plugins after Dispose()
	defer m.working.Set(true)

	return m.spawnEachPlugin("Dispose", func(i int) error {
		// This can be nil, if we're disposing after having failed to create all
		// taskPlugins. Say we had an error in NewTaskPlugin() for a plugin.
		if m.taskPlugins[i] == nil {
			return nil
		}
		err := m.taskPlugins[i].Dispose()
		// Errors are not allowed from Dispose()
		if err != nil && err != runtime.ErrFatalInternalError && err != runtime.ErrNonFatalInternalError {
			incidentID := m.monitors[i].WithTag("hook", "Dispose").ReportError(err, "Dispose() may not return errors")
			m.context.LogError("Unhandled worker error encountered incidentID=", incidentID)
			err = runtime.ErrFatalInternalError
		}
		return err
	})
}
