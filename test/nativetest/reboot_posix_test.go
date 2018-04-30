// +build linux,native darwin,native

package nativetest

import (
	"testing"

	"github.com/taskcluster/taskcluster-worker/worker/workertest"
)

func TestRebootAlways(t *testing.T) {
	workertest.Case{
		Engine:            "native",
		Concurrency:       0, // require tasks be sequantially dependent
		EngineConfig:      engineConfig,
		PluginConfig:      pluginConfig,
		StoppedGracefully: true, // Expect worker to stop on it's own
		Tasks: workertest.Tasks([]workertest.Task{{
			Title:   "Success",
			Success: true,
			Payload: `{
				"command": ["true"],
				"env": {},
				"maxRunTime": 30,
				"reboot": "on-failure"
			}`,
			AllowAdditional: true,
		}, {
			Title:   "Failure",
			Success: false,
			Payload: `{
				"command": ["false"],
				"env": {},
				"maxRunTime": 30,
				"reboot": "on-exception"
			}`,
			AllowAdditional: true,
		}, {
			Title:   "Reboot",
			Success: true,
			Payload: `{
				"command": ["true"],
				"env": {},
				"maxRunTime": 30,
				"reboot": "always"
			}`,
			AllowAdditional: true,
		}}),
	}.Test(t)
}

func TestRebootFailure(t *testing.T) {
	workertest.Case{
		Engine:            "native",
		Concurrency:       0, // require tasks be sequantially dependent
		EngineConfig:      engineConfig,
		PluginConfig:      pluginConfig,
		StoppedGracefully: true, // Expect worker to stop on it's own
		Tasks: workertest.Tasks([]workertest.Task{{
			Title:   "Success",
			Success: true,
			Payload: `{
				"command": ["true"],
				"env": {},
				"maxRunTime": 30,
				"reboot": "on-failure"
			}`,
			AllowAdditional: true,
		}, {
			Title:   "Failure",
			Success: false,
			Payload: `{
				"command": ["false"],
				"env": {},
				"maxRunTime": 30,
				"reboot": "on-exception"
			}`,
			AllowAdditional: true,
		}, {
			Title:   "Reboot on Failure",
			Success: false,
			Payload: `{
				"command": ["false"],
				"env": {},
				"maxRunTime": 30,
				"reboot": "on-failure"
			}`,
			AllowAdditional: true,
		}}),
	}.Test(t)
}
