// Package env provides a taskcluster-worker plugin that injects environment
// variables into the task environments.
//
// This plugin supports per-task environment variables specified in
// task.payload.env, but also globally configured environment variables, which
// can be used to inject information such as instance type.
//
// Finally, this plugin will inject TASK_ID and RUN_ID as environment variables.
package env

import (
	"strconv"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

type plugin struct {
	plugins.PluginBase
	extraVars map[string]string
}

type payload struct {
	Env map[string]string `json:"env"`
}

type config struct {
	Extra map[string]string `json:"extra"`
}

type provider struct {
	plugins.PluginProviderBase
}

type taskPlugin struct {
	plugins.TaskPluginBase
	variables map[string]string
}

func init() {
	plugins.Register("env", &provider{})
}

func (p *provider) ConfigSchema() schematypes.Schema {
	return schematypes.Object{
		Properties: schematypes.Properties{
			"extra": schematypes.Map{
				MetaData: schematypes.MetaData{
					Title: "Extra Environment Variables",
					Description: util.Markdown(`
						The 'extra' property holds a mapping from variable name to value.

						These _extra_ environment variables will be injected into all tasks,
						though they can be overwritten on per-task basis using the
						'task.payload.env' property.

						Notice that these overwrite built-in environment variables
						'TASK_ID' and 'RUN_ID' which is also supplied by this plugin.
					`),
				},
				Values: schematypes.String{},
			},
		},
	}
}

func (p *provider) NewPlugin(options plugins.PluginOptions) (plugins.Plugin, error) {
	var c config
	schematypes.MustValidateAndMap(p.ConfigSchema(), options.Config, &c)

	return &plugin{
		extraVars: c.Extra,
	}, nil
}

func (p *plugin) PayloadSchema() schematypes.Object {
	return schematypes.Object{
		Properties: schematypes.Properties{
			"env": schematypes.Map{
				MetaData: schematypes.MetaData{
					Title:       "Environment Variables",
					Description: "Mapping from environment variables to values",
				},
				Values: schematypes.String{},
			},
		},
	}
}

func (p *plugin) NewTaskPlugin(options plugins.TaskPluginOptions) (plugins.TaskPlugin, error) {
	var P payload
	schematypes.MustValidateAndMap(p.PayloadSchema(), options.Payload, &P)

	env := make(map[string]string)

	// Set built-in environment variables
	env["TASK_ID"] = options.TaskContext.TaskID
	env["RUN_ID"] = strconv.Itoa(options.TaskContext.RunID)

	// Set variables as configured globally
	for k, v := range p.extraVars {
		env[k] = v
	}
	// Set variables as configured per-task (overwriting globally config vars)
	for k, v := range P.Env {
		env[k] = v
	}

	return &taskPlugin{
		variables: env,
	}, nil
}

func (p *taskPlugin) BuildSandbox(sandboxBuilder engines.SandboxBuilder) error {
	for k, v := range p.variables {
		err := sandboxBuilder.SetEnvironmentVariable(k, v)

		// We can only return MalFormedPayloadError
		switch err {
		case engines.ErrNamingConflict:
			return runtime.NewMalformedPayloadError("Environment variable ", k, " has already been set.")
		case engines.ErrFeatureNotSupported:
			return runtime.NewMalformedPayloadError(
				"Custom environment variables are not supported in the current configuration of the engine")
		case nil:
			// break
		default:
			return err
		}
	}

	return nil
}
