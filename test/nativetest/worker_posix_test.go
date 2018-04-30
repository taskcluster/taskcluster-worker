// +build linux,native darwin,native

package nativetest

import (
	"testing"

	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/worker/workertest"
)

func TestWorker(t *testing.T) {
	workertest.Case{
		Engine:       "native",
		Concurrency:  1,
		EngineConfig: engineConfig,
		PluginConfig: pluginConfig,
		Tasks: workertest.Tasks([]workertest.Task{{
			Title:     "Invalid Command",
			Exception: runtime.ReasonMalformedPayload,
			Payload: `{
				"command": ["-definitely-an-invalid-command-"],
				"env": {},
				"maxRunTime": 30
			}`,
			AllowAdditional: true,
			Artifacts: workertest.ArtifactAssertions{
				// Expect error message that contains the command
				"public/logs/live_backing.log": workertest.GrepArtifact("-definitely-an-invalid-command-"),
			},
		}, {
			Title:     "JSON Schema Violation",
			Exception: runtime.ReasonMalformedPayload,
			Payload: `{
				"command": "should be an array",
				"env": {},
				"maxRunTime": 30
			}`,
			AllowAdditional: true,
			Artifacts: workertest.ArtifactAssertions{
				// Expect error message that contains the JSON path to violation
				"public/logs/live_backing.log": workertest.GrepArtifact("task.payload.command"),
			},
		}}),
	}.Test(t)
}
