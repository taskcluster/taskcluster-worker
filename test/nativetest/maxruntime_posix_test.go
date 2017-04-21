// +build linux,native darwin,native

package nativetest

import (
	"testing"

	"github.com/taskcluster/taskcluster-worker/worker/workertest"
)

func TestMaxRunTime(t *testing.T) {
	maxRunTimeCase.Test(t)
}

var maxRunTimeCase = workertest.Case{
	Engine:      "native",
	Concurrency: 1,
	EngineConfig: `{
		"createUser": false
	}`,
	PluginConfig: `{
		"disabled": [],
		"artifacts": {},
		"env": {},
		"maxruntime": {
			"perTaskLimit": "require",
			"maxRunTime": "3 hours"
		},
		"livelog": {},
		"success": {}
	}`,
	Tasks: []workertest.Task{
		{
			Title:   "Success",
			Success: true,
			Payload: `{
				"command": ["true"],
				"env": {},
				"maxRunTime": 30
			}`,
			AllowAdditional: true,
		},
		{
			Title:   "Failure",
			Success: false,
			Payload: `{
				"command": ["false"],
				"env": {},
				"maxRunTime": 30
			}`,
			AllowAdditional: true,
		},
		{
			Title:   "MaxRunTime Exceeded",
			Success: false,
			Payload: `{
				"command": ["sleep", "10s"],
				"env": {},
				"maxRunTime": 1
			}`,
			AllowAdditional: true,
			Artifacts: workertest.ArtifactAssertions{
				// We expect some error message mentioning maxRunTime
				"public/logs/live_backing.log": workertest.GrepArtifact("maxRunTime"),
			},
		},
	},
}

/*
 - folder with multiple artifacts, and sub-folders
 - content type for folder artifacts
 - file artifact is a folder
 + task can fail
 + task can succeed
 + maxRunTime kills
 - canceled task
 - test malformed payload

*/
