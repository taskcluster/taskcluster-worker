package scripttest

import (
	"testing"

	"github.com/taskcluster/taskcluster-worker/worker/workertest"
)

func TestWorker(t *testing.T) {
	workertest.Case{
		Engine:       "script",
		Concurrency:  1,
		EngineConfig: engineConfig,
		PluginConfig: pluginConfig,
		Tasks: []workertest.Task{{
			Title:           "successful task",
			Success:         true,
			Payload:         `{"result": "pass"}`,
			AllowAdditional: true,
		}, {
			Title:           "failing task",
			Success:         false,
			Payload:         `{"result": "fail"}`,
			AllowAdditional: true,
		}},
	}.Test(t)
}
