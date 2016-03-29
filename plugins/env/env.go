// This plugins creates the environment variables setup in the payload
// env" section. It does its processing at BuildSandbox call. After the
// "BuildSandox" stage, all environment variables configured in the payload
// are available in the SandboxBuilder.

package env

import (
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/plugins/extpoints"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type plugin struct {
	plugins.PluginBase
}

func (plugin) PayloadSchema() (runtime.CompositeSchema, error) {
	return envPayloadSchema, nil
}

func (plugin) NewTaskPlugin(options plugins.TaskPluginOptions) (plugins.TaskPlugin, error) {
	if options.Payload == nil {
		return plugins.TaskPluginBase{}, nil
	}
	return taskPlugin{
		TaskPluginBase: plugins.TaskPluginBase{},
		payload:        *(options.Payload.(*envPayload)),
	}, nil
}

type taskPlugin struct {
	plugins.TaskPluginBase
	payload envPayload
}

func (self taskPlugin) BuildSandbox(sandboxBuilder engines.SandboxBuilder) error {
	for k, v := range self.payload {
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
}

func (pluginProvider) NewPlugin(extpoints.PluginOptions) (plugins.Plugin, error) {
	return plugin{}, nil
}

func (pluginProvider) ConfigSchema() runtime.CompositeSchema {
	return runtime.NewEmptyCompositeSchema()
}

func init() {
	extpoints.PluginProviders.Register(new(pluginProvider), "env")
}
