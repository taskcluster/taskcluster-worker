// +build linux,docker

package dockertest

import (
	"testing"

	"github.com/taskcluster/taskcluster-worker/worker/workertest"
)

func TestEnv(t *testing.T) {
	workertest.Case{
		Engine:       "docker",
		Concurrency:  1,
		EngineConfig: engineConfig,
		PluginConfig: pluginConfig,
		Tasks: workertest.Tasks([]workertest.Task{{
			Title:   "Access Env Vars",
			Success: true,
			Payload: `{
				"image": "` + dockerImageName + `",
				"command": ["sh", "-c", "echo $MY_ENV_VAR"],
				"env": {
					"MY_ENV_VAR": "hello-world"
				},
				"maxRunTime": "10 minutes"
			}`,
			AllowAdditional: true, // Ignore additional artifacts
			Artifacts: workertest.ArtifactAssertions{
				"public/logs/live_backing.log": workertest.GrepArtifact("hello-world"),
			},
		}, {
			Title:   "TASK_ID and RUN_ID",
			Success: true,
			Payload: `{
				"image": "` + dockerImageName + `",
				"command": ["sh", "-c", "test -n \"$TASK_ID\" && test \"$RUN_ID\" = 0"],
				"env": {},
				"maxRunTime": "10 minutes"
			}`,
			AllowAdditional: true, // Ignore additional artifacts
		}}),
	}.Test(t)
}
