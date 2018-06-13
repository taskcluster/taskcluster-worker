// +build dockervideo

package dockerengine

import (
	"testing"

	"github.com/taskcluster/taskcluster-worker/engines/enginetest"
)

// Image and tag used in test cases below
const (
	videoDockerImageName = "alpine:3.6"
)

var videoProvider = &enginetest.EngineProvider{
	Engine: "docker",
	Config: `{
		"privileged": "allow",
		"enableDevices": true
	}`,
}

func TestVideo(t *testing.T) {
	c := enginetest.LoggingTestCase{
		EngineProvider: videoProvider,
		Target:         "/dev/video0",
		TargetPayload: `{
			"command": ["sh", "-c", "ls /dev/video0"],
			"devices": ["video"],
			"image": "` + videoDockerImageName + `"
		}`,
	}

	c.Test()
}
