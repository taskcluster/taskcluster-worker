// This plugins creates the environment variables setup in the payload
// env" section. It does its processing at BuildSandbox call. After the
// "BuildSandox" stage, all environment variables configured in the payload
// are available in the SandboxBuilder.

package env

import (
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
)

type plugin struct {
	plugins.PluginBase
}

type payloadType struct {
	Env map[string]string `json:"env"`
}

var payloadSchema = schematypes.Object{
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

func (plugin) PayloadSchema() schematypes.Object {
	return payloadSchema
}

func (plugin) NewTaskPlugin(options plugins.TaskPluginOptions) (plugins.TaskPlugin, error) {
	var p payloadType
	err := payloadSchema.Map(options.Payload, &p)
	if err == schematypes.ErrTypeMismatch {
		panic("internal error -- type mismatch")
	} else if err != nil {
		return nil, engines.ErrContractViolation
	}

	return taskPlugin{
		TaskPluginBase: plugins.TaskPluginBase{},
		variables:      p.Env,
	}, nil
}

type taskPlugin struct {
	plugins.TaskPluginBase
	variables map[string]string
}

func (p taskPlugin) BuildSandbox(sandboxBuilder engines.SandboxBuilder) error {
	for k, v := range p.variables {
		err := sandboxBuilder.SetEnvironmentVariable(k, v)

		// We can only return MalFormedPayloadError
		switch err {
		case engines.ErrNamingConflict:
			return engines.NewMalformedPayloadError("Environment variable ", k, " has already been set.")
		case engines.ErrFeatureNotSupported:
			return engines.NewMalformedPayloadError(
				"Cannot set environment variable ",
				k,
				". Engine does not support this operation")
		case nil:
			// break
		default:
			return err
		}
	}

	return nil
}

type pluginProvider struct {
	plugins.PluginProviderBase
}

func (pluginProvider) NewPlugin(plugins.PluginOptions) (plugins.Plugin, error) {
	return plugin{}, nil
}

func init() {
	plugins.RegisterPlugin("env", pluginProvider{})
}
