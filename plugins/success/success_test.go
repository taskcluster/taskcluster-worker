package success

import (
	"testing"

	"github.com/taskcluster/taskcluster-worker/plugins/plugintest"
)

func TestSuccessSuccessPlugin(*testing.T) {
	plugintest.Case{
		Payload: `{
			"delay": 0,
			"function": "true",
			"argument": "whatever"
		}`,
		Plugin:        "success",
		PluginSuccess: true,
		EngineSuccess: true,
	}.Test()
}

func TestSuccessFailurePlugin(*testing.T) {
	plugintest.Case{
		Payload: `{
			"delay": 0,
			"function": "false",
			"argument": "whatever"
		}`,
		Plugin:        "success",
		PluginSuccess: false,
		EngineSuccess: false,
	}.Test()
}
