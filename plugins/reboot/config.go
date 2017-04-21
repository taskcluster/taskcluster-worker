package reboot

import (
	"math"
	"time"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

type config struct {
	MaxLifeCycle     time.Duration `json:"maxLifeCycle"`
	TaskLimit        int           `json:"taskLimit"`
	AllowTaskReboots bool          `json:"allowTaskReboots"`
	RebootCommand    []string      `json:"rebootCommand"`
}

var configSchema = schematypes.Object{
	MetaData: schematypes.MetaData{
		Title: "Reboot Plugin",
		Description: util.Markdown(`
			The 'reboot' plugin can be configured to stop the worker gracefully, or
			to allow tasks to stop the worker gracefully.

			The 'reboot' plugin assumes the worker is deployed in an environment where
			host machine reboots or resets if the worker exits. Hence, stopping
			gracefully is equivalent to rebooting the worker.

			If this behaviour is not the case, the 'reboot' plugin also allow
			configuration of an optional command to be executed when the worker is
			terminating due to a graceful shutdown initiated by the 'reboot' plugin.
		`),
	},
	Properties: schematypes.Properties{
		"maxLifeCycle": schematypes.Duration{
			MetaData: schematypes.MetaData{
				Title: "Max Worker Life-Cycle",
				Description: util.Markdown(`
					Maximum amount of time before gracefully shutting down the worker.

					Given as integer in seconds or as string on the form:
					'1 day 2 hours 3 minutes'. Leave the value as zero or empty string to
					disable worker life-cycle limitation.
				`),
			},
		},
		"taskLimit": schematypes.Integer{
			MetaData: schematypes.MetaData{
				Title: "Task Limit",
				Description: util.Markdown(`
					Maximum number of tasks to process before gracefully shutting down the
					worker.

					Defaults to zero implying no limitation. This is mainly useful as
					'taskLimit: 1' to reboot between each task. Or if running tests and
					you want to stop the worker after one task.
				`),
			},
			Minimum: 0,
			Maximum: math.MaxInt64,
		},
		"allowTaskReboots": schematypes.Boolean{
			MetaData: schematypes.MetaData{
				Title: "Allow Task Reboots",
				Description: util.Markdown(`
					Allow tasks to specify a 'task.payload.reboot' that specifies if the
					worker should _reboot_ after the task finishes.

					Tasks can specify 'task.payload.reboot' as '"always"', '"on-failure"'
					and '"on-exception"', the property is always optional.
				`),
			},
		},
		"rebootCommand": schematypes.Array{
			MetaData: schematypes.MetaData{
				Title: "Reboot Command",
				Description: util.Markdown(`
					Command to run when the worker is stopping, if the reboot plugin
					caused the worker to stop.

					It is recommended that the worker is launched by a start-up script
					that reboots/resets the system and lunches the worker again, when the
					worker exits. This is a fairly robust behavior on most deployment
					scenarios.

					However, there may be cases where it's desirable to change behavior
					depending on whether or not it is the 'reboot' plugin that stops the
					worker. In these cases this _optional_ command can be useful.

					The command is executed when the worker is stopping, if the 'reboot'
					plugin initiated the worker shutdown. The command will be executed
					after all tasks have finished as part of the clean-up process, hence,
					one can specify '["sudo", "reboot"]' and expect a graceful reboot.
				`),
			},
			Items: schematypes.String{},
		},
	},
}
