package maxruntime

import (
	"testing"

	"github.com/taskcluster/taskcluster-worker/plugins/plugintest"
)

func TestMaxRunTimeSuccess(t *testing.T) {
	plugintest.Case{
		Payload: `{
			"delay": 100,
			"function": "true",
			"argument": "whatever",
			"maxRunTime": "1 minute"
		}`,
		Plugin: "maxruntime",
		PluginConfig: `{
			"maxRunTime": "10 minute",
			"perTaskLimit": "require"
		}`,
		PluginSuccess: true,
		EngineSuccess: true,
	}.Test()
}

func TestMaxRunTimeExpired(t *testing.T) {
	plugintest.Case{
		Payload: `{
			"delay": 10000,
			"function": "true",
			"argument": "whatever",
			"maxRunTime": 1
		}`,
		Plugin: "maxruntime",
		PluginConfig: `{
			"maxRunTime": "1 minute",
			"perTaskLimit": "allow"
		}`,
		PluginSuccess: false,
		EngineSuccess: false,
	}.Test()
}
