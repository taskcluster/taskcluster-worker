// This plugin allows a task to reboot the machine after it is finished.
// It must add the boolean "reboot" payload attribute.

package reboot

import (
	"fmt"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/plugins"
)

type plugin struct {
	plugins.PluginBase
}

type payloadType struct {
	Reboot bool `json:"reboot"`
}

type taskPlugin struct {
	plugins.TaskPluginBase
	reboot bool
}

type pluginProvider struct {
	plugins.PluginProviderBase
}

var payloadSchema = schematypes.Object{
	Properties: schematypes.Properties{
		"reboot": schematypes.Boolean{
			MetaData: schematypes.MetaData{
				Title:       "Reboot machine",
				Description: "If true, reboot the machine after task is finished.",
			},
		},
	},
}

func (plugin) PayloadSchema() schematypes.Object {
	return payloadSchema
}

func (plugin) NewTaskPlugin(options plugins.TaskPluginOptions) (plugins.TaskPlugin, error) {
	var p payloadType
	schematypes.MustValidateAndMap(payloadSchema, options.Payload, &p)

	return taskPlugin{
		TaskPluginBase: plugins.TaskPluginBase{},
		reboot:         p.Reboot,
	}, nil
}

func (tp taskPlugin) Dispose() error {
	if tp.reboot {
		if err := reboot(); err != nil {
			return fmt.Errorf("Failed to force reboot, error: %s", err)
		}
	}

	return nil
}

func (pluginProvider) NewPlugin(options plugins.PluginOptions) (plugins.Plugin, error) {
	return plugin{}, nil
}

func init() {
	plugins.Register("reboot", pluginProvider{})
}
