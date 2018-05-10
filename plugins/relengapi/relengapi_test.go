package relengapi

import (
	"testing"

	"github.com/taskcluster/taskcluster-worker/plugins/plugintest"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
)

func TestTCProxySuccess(t *testing.T) {
	plugintest.Case{
		Payload: `{
			"delay": 100,
			"function": "ping-proxy",
			"argument": "http://relengapi/versions",
			"enableRelengAPIProxy": true
		}`,
		Plugin: "relengapi",
		PluginConfig: `{
			"token": "1234567890"
		}`,
		ClientID:      "tester",
		AccessToken:   "no-secret",
		PluginSuccess: true,
		EngineSuccess: true,
		TaskID:        "1234",
	}.Test()
}

func TestTCProxyFail(t *testing.T) {
	plugintest.Case{
		Payload: `{
			"delay": 100,
			"function": "ping-proxy",
			"argument": "http://relengapi/foo/bar",
			"enableRelengAPIProxy": true
		}`,
		Plugin: "relengapi",
		PluginConfig: `{
			"token": "1234567890"
		}`,
		ClientID:      "tester",
		AccessToken:   "wrong-secret",
		PluginSuccess: true,
		EngineSuccess: false,
		TaskID:        "1234",
		QueueMock:     new(client.MockQueue),
	}.Test()
}
