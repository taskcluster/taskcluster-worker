// +build linux,docker

package dockertest

import (
	"testing"

	"github.com/taskcluster/taskcluster-worker/worker/workertest"
)

func TestMaxRunTime(t *testing.T) {
	workertest.Case{
		Engine:       "docker",
		Concurrency:  1,
		EngineConfig: engineConfig,
		PluginConfig: pluginConfig,
		Tasks: workertest.Tasks([]workertest.Task{{
			Title:   "Success",
			Success: true,
			Payload: `{
				"image": "` + dockerImageName + `",
				"command": ["true"],
				"env": {},
				"maxRunTime": 30
			}`,
			AllowAdditional: true,
		}, {
			Title:   "Failure",
			Success: false,
			Payload: `{
				"image": "` + dockerImageName + `",
				"command": ["false"],
				"env": {},
				"maxRunTime": 30
			}`,
			AllowAdditional: true,
		}, {
			Title:   "MaxRunTime Exceeded",
			Success: false,
			Payload: `{
				"image": "` + dockerImageName + `",
				"command": ["sleep", "10s"],
				"env": {},
				"maxRunTime": 1
			}`,
			AllowAdditional: true,
			Artifacts: workertest.ArtifactAssertions{
				// We expect some error message mentioning maxRunTime
				"public/logs/live_backing.log": workertest.GrepArtifact("maxRunTime"),
			},
		}}),
	}.Test(t)
}
