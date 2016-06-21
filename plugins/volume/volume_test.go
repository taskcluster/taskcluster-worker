package volume

import (
	"testing"

	"github.com/taskcluster/taskcluster-worker/plugins/plugintest"
)

type volumeTestCase struct {
	plugintest.Case
}

func TestVolumeValidType(t *testing.T) {
	volumeTestCase{
		Case: plugintest.Case{
			Payload: `{
				"start": {
					"delay": 10,
					"function": "set-volume",
					"argument": "/home/worker"
				},
				"volumes": {
					"persistent": [
						{
							"mountPoint": "/home/worker",
							"name": "test-workspace"
						}
					]
				}
			}`,
			Plugin:        "volume",
			TestStruct:    t,
			PluginSuccess: true,
			EngineSuccess: true,
		},
	}.Test()
}
