package tcproxy

import (
	"testing"

	"github.com/taskcluster/taskcluster-worker/plugins/plugintest"
)

func TestTCProxySuccess(t *testing.T) {
	plugintest.Case{
		Payload: `{
			"delay": 100,
			"function": "ping-proxy",
			"argument": "http://tcproxy/auth.taskcluster.net/v1/test-authenticate-get"
		}`,
		Plugin:        "tcproxy",
		ClientID:      "tester",
		AccessToken:   "no-secret",
		PluginSuccess: true,
		EngineSuccess: true,
	}.Test()
}

func TestTCProxyFail(t *testing.T) {
	plugintest.Case{
		Payload: `{
			"delay": 100,
			"function": "ping-proxy",
			"argument": "http://tcproxy/auth.taskcluster.net/v1/test-authenticate-get"
		}`,
		Plugin:        "tcproxy",
		ClientID:      "tester",
		AccessToken:   "wrong-secret",
		PluginSuccess: true,
		EngineSuccess: false,
	}.Test()
}
