// This plugins creates the environment variables setup in the payload
// env" section. It does its processing at BuildSandbox call. After the
// "BuildSandox" stage, all environment variables configured in the payload
// are available in the SandboxBuilder.

package env

import (
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type plugin struct {
	plugins.PluginBase
	extraVars map[string]string
}

type payloadType struct {
	Env map[string]string `json:"env"`
}

type config struct {
	Extra map[string]string `json:"extra"`
}

var configSchema = schematypes.Object{
	Properties: schematypes.Properties{
		"extra": schematypes.Map{
			MetaData: schematypes.MetaData{
				Title:       "Extra environment variables",
				Description: "This defines extra environment variables to add to the engine.",
			},
			Values: schematypes.String{},
		},
	},
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

func (*plugin) PayloadSchema() schematypes.Object {
	return payloadSchema
}

func (pl *plugin) NewTaskPlugin(options plugins.TaskPluginOptions) (plugins.TaskPlugin, error) {
	p := &payloadType{
		// Must explicitly create an empty map, so if payload does not include
		// env vars, we'll still have a valid map to read from/write to.
		Env: map[string]string{},
	}
	err := payloadSchema.Map(options.Payload, p)
	if err == schematypes.ErrTypeMismatch {
		panic("internal error -- type mismatch")
	} else if err != nil {
		return nil, engines.ErrContractViolation
	}

	for k, v := range pl.extraVars {
		p.Env[k] = v
	}

	return &taskPlugin{
		TaskPluginBase: plugins.TaskPluginBase{},
		variables:      p.Env,
	}, nil
}

type taskPlugin struct {
	plugins.TaskPluginBase
	variables map[string]string
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

func (*pluginProvider) NewPlugin(options plugins.PluginOptions) (plugins.Plugin, error) {
	var c config
	if err := schematypes.MustMap(configSchema, options.Config, &c); err != nil {
		return nil, engines.ErrContractViolation
	}

	return &plugin{
		PluginBase: plugins.PluginBase{},
		extraVars:  c.Extra,
	}, nil
}

func (*pluginProvider) ConfigSchema() schematypes.Schema {
	return configSchema
}

func init() {
	plugins.Register("env", &pluginProvider{})
}
