package workertest

import (
	"testing"

	"github.com/taskcluster/taskcluster-worker/runtime"

	_ "github.com/taskcluster/taskcluster-worker/engines/mock"
	_ "github.com/taskcluster/taskcluster-worker/plugins/env"
	_ "github.com/taskcluster/taskcluster-worker/plugins/livelog"
	_ "github.com/taskcluster/taskcluster-worker/plugins/success"
)

func TestWorkerTest(t *testing.T) {
	Case{
		Engine:       "mock",
		Concurrency:  2,
		EngineConfig: `{}`,
		PluginConfig: `{
			"disabled": [],
			"success": {}
		}`,
		Tasks: Tasks([]Task{
			{
				Title:   "Task Success",
				Success: true,
				Payload: `{
					"delay": 50,
					"function": "true",
					"argument": ""
				}`,
			}, {
				Title:   "Task Failure",
				Success: false,
				Payload: `{
					"delay": 50,
					"function": "false",
					"argument": ""
				}`,
			},
		}),
	}.Test(t)
}

func TestWorkerWithEnv(t *testing.T) {
	Case{
		Engine:       "mock",
		Concurrency:  1,
		EngineConfig: `{}`,
		PluginConfig: `{
			"disabled": [],
			"env": {
				"extra": {
				"STATIC_ENV_VAR": "static-value"
				}
			},
			"livelog": {},
			"success": {}
		}`,
		Tasks: Tasks([]Task{
			{
				Title:   "Print Task Env Var",
				Success: true,
				Payload: `{
					"delay": 50,
					"function": "print-env-var",
					"argument": "HELLO_WORLD",
					"env": {
						"HELLO_WORLD": "hello-world"
					}
				}`,
				Artifacts: ArtifactAssertions{
					"public/logs/live_backing.log": GrepArtifact("hello-world"),
					"public/logs/live.log":         AnyArtifact(),
				},
			}, {
				Title:   "Print Static Env Var",
				Success: true,
				Payload: `{
					"delay": 50,
					"function": "print-env-var",
					"argument": "STATIC_ENV_VAR"
				}`,
				Artifacts: ArtifactAssertions{
					"public/logs/live_backing.log": GrepArtifact("static-value"),
					"public/logs/live.log":         AnyArtifact(),
				},
			},
		}),
	}.Test(t)
}

func TestWorkerMalformedPayload(t *testing.T) {
	// This is a special test case, because the task.payload doesn't match the
	// schema. Hence, we can declare malformed-payload before running anything.
	// However, we still want to PluginManager.NewTaskPlugins to run and create
	// a livelog TaskPlugin, otherwise no log with an explanation of the schema
	// error won't be uploaded.
	Case{
		Engine:       "mock",
		Concurrency:  2,
		EngineConfig: `{}`,
		PluginConfig: `{
			"disabled": [],
			"success": {},
			"livelog": {}
		}`,
		Tasks: Tasks([]Task{
			{
				Title:     "Task Malformed Payload",
				Exception: runtime.ReasonMalformedPayload,
				Payload:   `{}`, // Should never create a SandboxBuilder
				Artifacts: ArtifactAssertions{
					// We expect an error message that says something about task.payload
					"public/logs/live_backing.log": GrepArtifact("task.payload"),
				},
				AllowAdditional: true,
			},
		}),
	}.Test(t)
}
