package scripttest

import (
	"testing"

	"github.com/taskcluster/taskcluster-worker/worker/workertest"
)

func TestLogging(t *testing.T) {
	workertest.Case{
		Engine:       "script",
		Concurrency:  1,
		EngineConfig: engineConfig,
		PluginConfig: pluginConfig,
		Tasks: []workertest.Task{{
			Title:           "hello-world pass",
			Success:         true,
			Payload:         `{"result": "pass", "message": "hello-world"}`,
			AllowAdditional: false,
			Artifacts: workertest.ArtifactAssertions{
				"public/logs/live.log":         workertest.ReferenceArtifact(),
				"public/logs/live_backing.log": workertest.GrepArtifact("hello-world"),
			},
		}, {
			Title:           "hello-world missing",
			Success:         true,
			Payload:         `{"result": "pass", "message": "hello-not-world"}`,
			AllowAdditional: false,
			Artifacts: workertest.ArtifactAssertions{
				"public/logs/live.log":         workertest.ReferenceArtifact(),
				"public/logs/live_backing.log": workertest.NotGrepArtifact("hello-world"),
			},
		}, {
			Title:           "hello-world fail",
			Success:         false,
			Payload:         `{"result": "fail", "message": "hello-world"}`,
			AllowAdditional: false,
			Artifacts: workertest.ArtifactAssertions{
				"public/logs/live.log":         workertest.ReferenceArtifact(),
				"public/logs/live_backing.log": workertest.GrepArtifact("hello-world"),
			},
		}, {
			Title:           "hello-world delay and pass",
			Success:         true,
			Payload:         `{"result": "pass", "message": "hello-world", "delay": 50}`,
			AllowAdditional: false,
			Artifacts: workertest.ArtifactAssertions{
				"public/logs/live.log":         workertest.ReferenceArtifact(),
				"public/logs/live_backing.log": workertest.GrepArtifact("hello-world"),
			},
		}},
	}.Test(t)
}
