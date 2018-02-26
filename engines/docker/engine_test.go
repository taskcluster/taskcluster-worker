package dockerengine

import (
	"testing"

	"github.com/taskcluster/taskcluster-worker/engines/enginetest"
)

var provider = &enginetest.EngineProvider{
	Engine: "docker",
	Config: `{
		"dockerEndpoint": "unix:///var/run/docker.sock",
		"maxConcurrency": 1
	}`,
}

func TestLogging(t *testing.T) {
	c := enginetest.LoggingTestCase{
		EngineProvider: provider,
		Target:         "hello-world",
		TargetPayload: `{
			"command": ["sh", "-c", "echo 'hello-world' && true"],
			"image": {
				"repository": "ubuntu",
				"tag": "latest"
			}
		}`,
		FailingPayload: `{
			"command": ["sh", "-c", "echo 'hello-world' && false"],
			"image": {
				"repository": "ubuntu",
				"tag": "latest"
			}
		}`,
		SilentPayload: `{
			"command": ["sh", "-c", "echo 'no hello' && true"],
			"image": {
				"repository": "ubuntu",
				"tag": "latest"
			}
		}`,
	}

	c.TestLogTarget()
	c.TestLogTargetWhenFailing()
	c.TestSilentTask()
	c.Test()
}
