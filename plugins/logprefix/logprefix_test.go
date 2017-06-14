package logprefix

import (
	"testing"

	"github.com/taskcluster/taskcluster-worker/plugins/plugintest"
)

func TestLogPrefixAddTaskID(*testing.T) {
	plugintest.Case{
		Payload: `{
			"delay": 0,
			"function": "true",
			"argument": ""
		}`,
		TaskID:        "Ghv98GSxQL2dR7eD8hXbMw",
		PluginConfig:  `{}`,
		Plugin:        "logprefix",
		PluginSuccess: true,
		EngineSuccess: true,
		MatchLog:      "Ghv98GSxQL2dR7eD8hXbMw",
	}.Test()
}

func TestLogPrefixAddTaskCount(*testing.T) {
	plugintest.Case{
		Payload: `{
			"delay": 0,
			"function": "true",
			"argument": ""
		}`,
		TaskID:        "Ghv98GSxQL2dR7eD8hXbMw",
		PluginConfig:  `{}`,
		Plugin:        "logprefix",
		PluginSuccess: true,
		EngineSuccess: true,
		MatchLog:      "TasksSinceStartup",
	}.Test()
}

func TestLogPrefixConfiguredKeys(*testing.T) {
	plugintest.Case{
		Payload: `{
			"delay": 0,
			"function": "true",
			"argument": ""
		}`,
		TaskID: "Ghv98GSxQL2dR7eD8hXbMw",
		PluginConfig: `{
			"CPU": "intel"
		}`,
		Plugin:        "logprefix",
		PluginSuccess: true,
		EngineSuccess: true,
		MatchLog:      "CPU",
	}.Test()
}

func TestLogPrefixConfiguredValues(*testing.T) {
	plugintest.Case{
		Payload: `{
			"delay": 0,
			"function": "true",
			"argument": ""
		}`,
		TaskID: "Ghv98GSxQL2dR7eD8hXbMw",
		PluginConfig: `{
			"CPU": "intel"
		}`,
		Plugin:        "logprefix",
		PluginSuccess: true,
		EngineSuccess: true,
		MatchLog:      "intel",
	}.Test()
}

func TestLogPrefixOverwriteBuiltin(*testing.T) {
	plugintest.Case{
		Payload: `{
			"delay": 0,
			"function": "true",
			"argument": ""
		}`,
		TaskID: "Ghv98GSxQL2dR7eD8hXbMw",
		PluginConfig: `{
			"TaskId": "fakeTaskId"
		}`,
		Plugin:        "logprefix",
		PluginSuccess: true,
		EngineSuccess: true,
		MatchLog:      "fakeTaskId",
		NotMatchLog:   "Ghv98GSxQL2dR7eD8hXbMw",
	}.Test()
}
