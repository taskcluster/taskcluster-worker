package interactive

import (
	"testing"

	"github.com/taskcluster/taskcluster-worker/plugins/plugintest"
)

func TestInteractivePluginDoingNothing(*testing.T) {
	plugintest.Case{
		Payload: `{
			"delay": 250,
			"function": "true",
			"argument": "whatever"
		}`,
		Plugin:        "interactive",
		PluginConfig:  `{}`,
		PluginSuccess: true,
		EngineSuccess: true,
	}.Test()
}
