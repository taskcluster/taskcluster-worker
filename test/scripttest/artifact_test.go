package scripttest

import (
	"testing"

	"github.com/taskcluster/taskcluster-worker/worker/workertest"
)

func TestArtifacts(t *testing.T) {
	workertest.Case{
		Engine:       "script",
		Concurrency:  1,
		EngineConfig: engineConfig,
		PluginConfig: pluginConfig,
		Tasks: []workertest.Task{{
			Title:           "hello-world artifact",
			Success:         true,
			Payload:         `{"result": "pass", "artifacts": {"public/build/test-output.txt": "hello-world"}}`,
			AllowAdditional: false,
			Artifacts: workertest.ArtifactAssertions{
				"public/logs/live.log":         workertest.AnyArtifact(),
				"public/logs/live_backing.log": workertest.AnyArtifact(),
				"public/build/test-output.txt": workertest.GrepArtifact("hello-world"),
			},
		}, {
			Title:           "empty artifact",
			Success:         true,
			Payload:         `{"result": "pass", "artifacts": {"public/build/test-output.txt": ""}}`,
			AllowAdditional: false,
			Artifacts: workertest.ArtifactAssertions{
				"public/logs/live.log":         workertest.AnyArtifact(),
				"public/logs/live_backing.log": workertest.AnyArtifact(),
				"public/build/test-output.txt": workertest.MatchArtifact("", "text/plain; charset=utf-8"),
			},
		}},
	}.Test(t)
}
