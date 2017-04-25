// +build linux,native darwin,native

package nativetest

import (
	"testing"

	"github.com/taskcluster/taskcluster-worker/worker/workertest"
)

func TestEnv(t *testing.T) {
	envCase.Test(t)
}

var envCase = workertest.Case{
	Engine:       "native",
	Concurrency:  1,
	EngineConfig: engineConfig,
	PluginConfig: pluginConfig,
	Tasks: []workertest.Task{
		{
			Title:   "Access Extra Env Vars",
			Success: true,
			Payload: `{
				"command": ["sh", "-c", "echo $MY_STATIC_VAR"],
				"env": {},
				"maxRunTime": "10 minutes"
			}`,
			AllowAdditional: true, // Ignore additional artifacts
			Artifacts: workertest.ArtifactAssertions{
				"public/logs/live_backing.log": workertest.GrepArtifact("static-value"),
			},
		},
		{
			Title:   "Access Env Vars",
			Success: true,
			Payload: `{
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
		},
		{
			Title:   "Overwrite Static Env Vars",
			Success: true,
			Payload: `{
				"command": ["sh", "-c", "echo $MY_STATIC_VAR"],
				"env": {
					"MY_STATIC_VAR": "hello-world"
				},
				"maxRunTime": "10 minutes"
			}`,
			AllowAdditional: true, // Ignore additional artifacts
			Artifacts: workertest.ArtifactAssertions{
				"public/logs/live_backing.log": workertest.GrepArtifact("hello-world"),
			},
		},
		{
			Title:   "TASK_ID and RUN_ID",
			Success: true,
			Payload: `{
				"command": ["sh", "-c", "test -n \"$TASK_ID\" && test \"$RUN_ID\" = 0"],
				"env": {},
				"maxRunTime": "10 minutes"
			}`,
			AllowAdditional: true, // Ignore additional artifacts
		},
	},
}
