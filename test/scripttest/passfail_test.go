package scripttest

import (
	"fmt"
	"testing"

	"github.com/taskcluster/taskcluster-worker/worker/workertest"
)

func TestPassFail(t *testing.T) {
	workertest.Case{
		Engine:       "script",
		Concurrency:  1,
		EngineConfig: engineConfig,
		PluginConfig: pluginConfig,
		Tasks: workertest.Tasks([]workertest.Task{{
			Title:           "successful task",
			Success:         true,
			Payload:         `{"result": "pass"}`,
			AllowAdditional: true,
		}, {
			Title:           "failing task",
			Success:         false,
			Payload:         `{"result": "fail"}`,
			AllowAdditional: true,
		}}),
	}.Test(t)
}

func TestManyPassTasks(t *testing.T) {
	workertest.Case{
		Engine:       "script",
		Concurrency:  1,
		EngineConfig: engineConfig,
		PluginConfig: pluginConfig,
		Tasks: func(t *testing.T, env workertest.Environment) []workertest.Task {
			var tasks []workertest.Task
			for i := 0; i < 10; i++ {
				tasks = append(tasks, workertest.Task{
					Title:           fmt.Sprintf("successful task %d", i+1),
					Success:         true,
					Payload:         `{"result": "pass"}`,
					AllowAdditional: true,
				})
			}
			return tasks
		},
	}.Test(t)
}

func TestManyFailTasks(t *testing.T) {
	workertest.Case{
		Engine:       "script",
		Concurrency:  1,
		EngineConfig: engineConfig,
		PluginConfig: pluginConfig,
		Tasks: func(t *testing.T, env workertest.Environment) []workertest.Task {
			var tasks []workertest.Task
			for i := 0; i < 10; i++ {
				tasks = append(tasks, workertest.Task{
					Title:           fmt.Sprintf("successful task %d", i+1),
					Success:         false,
					Payload:         `{"result": "fail"}`,
					AllowAdditional: true,
				})
			}
			return tasks
		},
	}.Test(t)
}
