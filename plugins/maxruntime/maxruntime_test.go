package maxruntime

import (
	"testing"

	"github.com/taskcluster/taskcluster-worker/plugins/plugintest"
)

func TestMaxRunRimeSuccess(t *testing.T) {
	plugintest.Case{
		Payload: `{
			"delay": 100,
			"function": "true",
			"argument": "whatever",
			"maxRunTime": 1
		}`,
		Plugin:        "maxruntime",
		PluginSuccess: true,
		EngineSuccess: true,
	}.Test()
}

func TestMaxRunRimeExpired(t *testing.T) {
	plugintest.Case{
		Payload: `{
			"delay": 10000,
			"function": "true",
			"argument": "whatever",
			"maxRunTime": 1
		}`,
		Plugin:        "maxruntime",
		PluginSuccess: true,
		EngineSuccess: false,
		SandboxAbort:  true,
	}.Test()
}
