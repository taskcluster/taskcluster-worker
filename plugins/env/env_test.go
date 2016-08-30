package env

import (
	"testing"

	"github.com/taskcluster/taskcluster-worker/plugins/plugintest"
)

func TestEnvNone(*testing.T) {
	plugintest.Case{
		Payload: `{
			"delay": 0,
			"function": "true",
			"argument": "whatever"
		}`,
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
		Plugin:        "env",
		PluginSuccess: true,
		EngineSuccess: false,
		NotMatchLog:   "env1",
	}.Test()
}
