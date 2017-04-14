package plugintest

import (
	"testing"

	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
)

type pluginProvider struct {
	plugins.PluginProviderBase
}

func (pluginProvider) NewPlugin(options plugins.PluginOptions) (plugins.Plugin, error) {
	assert(options.Monitor != nil, "PluginOptions.Monitor is nil!")
	e := options.Environment
	assert(e.GarbageCollector != nil, "PluginOptions.Environment.GarbageCollector is nil!")
	assert(e.Monitor != nil, "PluginOptions.Environment.Monitor is nil!")
	assert(e.TemporaryStorage != nil, "PluginOptions.Environment.TemporaryStorage is nil!")
	assert(e.WebHookServer != nil, "PluginOptions.Environment.WebHookServer is nil!")
	return plugin{}, nil
}

type plugin struct {
	plugins.PluginBase
}

type taskPlugin struct {
	plugins.TaskPluginBase
}

func init() {
	plugins.Register("plugintest-tester", pluginProvider{})
}

func (plugin) NewTaskPlugin(options plugins.TaskPluginOptions) (plugins.TaskPlugin, error) {
	assert(options.Monitor != nil, "TaskPluginOptions.Monitor is nil!")
	assert(options.TaskContext != nil, "TaskPluginOptions.TaskContext is nil!")
	return taskPlugin{}, nil
}

func (taskPlugin) Stopped(result engines.ResultSet) (bool, error) {
	assert(result != nil, "Expected a resultset")
	return true, nil
}

func TestSuccessSuccessPlugin(*testing.T) {
	Case{
		Payload: `{
			"delay": 0,
			"function": "true",
			"argument": "whatever"
		}`,
		Plugin:        "plugintest-tester",
		PluginSuccess: true,
		EngineSuccess: true,
	}.Test()
}
