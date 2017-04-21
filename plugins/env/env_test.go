package env

import (
	"testing"

	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-worker/plugins/plugintest"
)

func TestEnvNone(*testing.T) {
	plugintest.Case{
		Payload: `{
			"delay": 0,
			"function": "true",
			"argument": "whatever"
		}`,
		PluginConfig:  `{}`,
		Plugin:        "env",
		PluginSuccess: true,
		EngineSuccess: true,
	}.Test()
}

func TestEnvDefinition(*testing.T) {
	plugintest.Case{
		Payload: `{
			"delay": 0,
			"function": "print-env-var",
			"argument": "ENV1",
			"env": {
				"ENV1": "env1"
			}
		}`,
		PluginConfig:  `{}`,
		Plugin:        "env",
		PluginSuccess: true,
		EngineSuccess: true,
		MatchLog:      "env1",
	}.Test()
}

func TestEnvUnDefinition(*testing.T) {
	plugintest.Case{
		Payload: `{
			"delay": 0,
			"function": "print-env-var",
			"argument": "ENV1",
			"env": {
				"ENV2": "env2"
			}
		}`,
		PluginConfig:  `{}`,
		Plugin:        "env",
		PluginSuccess: true,
		EngineSuccess: false,
		NotMatchLog:   "env1",
	}.Test()
}

func TestEnvConfig(*testing.T) {
	plugintest.Case{
		Payload: `{
			"delay": 0,
			"function": "print-env-var",
			"argument": "ENV1",
			"env": {
				"ENV2" : "env2"
			}
		}`,
		PluginConfig: `{
			"extra": {
				"ENV1": "env1"
			}
		}`,
		Plugin:        "env",
		PluginSuccess: true,
		EngineSuccess: true,
		MatchLog:      "env1",
	}.Test()
}

func TestEnvOverwrites(*testing.T) {
	plugintest.Case{
		Payload: `{
			"delay": 0,
			"function": "print-env-var",
			"argument": "ENV1",
			"env": {
				"ENV1" : "env2"
			}
		}`,
		PluginConfig: `{
			"extra": {
				"ENV1": "env1"
			}
		}`,
		Plugin:        "env",
		PluginSuccess: true,
		EngineSuccess: true,
		MatchLog:      "env2",
	}.Test()
}

func TestInjectsTaskID(*testing.T) {
	TaskID := slugid.Nice()
	plugintest.Case{
		TaskID: TaskID,
		Payload: `{
			"delay": 0,
			"function": "print-env-var",
			"argument": "TASK_ID"
		}`,
		PluginConfig:  `{}`,
		Plugin:        "env",
		PluginSuccess: true,
		EngineSuccess: true,
		MatchLog:      TaskID,
	}.Test()
}

func TestInjectsRunID(*testing.T) {
	plugintest.Case{
		RunID: 7,
		Payload: `{
			"delay": 0,
			"function": "print-env-var",
			"argument": "RUN_ID"
		}`,
		PluginConfig:  `{}`,
		Plugin:        "env",
		PluginSuccess: true,
		EngineSuccess: true,
		MatchLog:      "7",
	}.Test()
}
