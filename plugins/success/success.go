// Package success implements a very simple plugin that looks that the
// ResultSet.Success() value to determine if the process from the sandbox
// exited successfully.
//
// Most engines implements ResultSet.Success() to mean the sub-process exited
// non-zero. In this plugin we use this in the Stopped() hook to ensure that
// tasks are declared "failed" if they had a non-zero exit code.
//
// The attentive reader might think this is remarkably simple and stupid plugin.
// This is true, but it does display the concept of plugins and more importantly
// removes a special case that we would otherwise have to take into
// consideration in the runtime.
package success

import (
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
)

type pluginProvider struct {
	plugins.PluginProviderBase
}

func (pluginProvider) NewPlugin(plugins.PluginOptions) (plugins.Plugin, error) {
	return plugin{}, nil
}

type plugin struct {
	plugins.PluginBase
}

type taskPlugin struct {
	plugins.TaskPluginBase
}

func init() {
	plugins.RegisterPlugin("success", pluginProvider{})
}

func (plugin) NewTaskPlugin(plugins.TaskPluginOptions) (plugins.TaskPlugin, error) {
	return taskPlugin{}, nil
}

func (taskPlugin) Stopped(result engines.ResultSet) (bool, error) {
	return result.Success(), nil
}
