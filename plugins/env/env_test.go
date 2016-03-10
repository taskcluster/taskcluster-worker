package env

import (
	"github.com/taskcluster/taskcluster-worker/plugins/plugintest"
	"testing"
)

func TestEnvDefinition(*testing.T) {
	plugintest.Case{
		Payload: `{
			"start": {
				"delay": 0,
				"function": "print-env-var",
				"argument": "ENV1"
			},
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
			"start": {
				"delay": 0,
				"function": "print-env-var",
				"argument": "ENV1"
			},
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
