package maxruntime

import (
	"time"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

type config struct {
	PerTaskLimit string        `json:"perTaskLimit"`
	MaxRunTime   time.Duration `json:"maxRunTime"`
}

const (
	limitRequire = "require"
	limitAllow   = "allow"
	limitForbid  = "forbid"
)

var configSchema = schematypes.Object{
	MetaData: schematypes.MetaData{
		Title: "`maxRunTime` Plugin",
		Description: util.Markdown(`
			The maximum runtime plugin kills tasks that have a runtime exceeding the
			'maxRunTime' specified. This means it kills the sandbox and any interactive
			shells and display. Once killed the task will be resolved as failed.

			This plugin only limits the time between execution start and end. That
			does not include artifact upload, download of images, etc. To guard
			against worker deadlocks use the 'watchdog' plugin.

			A 'maxRunTime' limit is given in the plugin configuration, and the
			'perTaskLimit' option can be used to allow or require tasks to specify a
			shorter 'maxRunTime'.
		`),
	},
	Properties: schematypes.Properties{
		"maxRunTime": schematypes.Duration{
			MetaData: schematypes.MetaData{
				Title: "Maximum Task Run-Time",
				Description: util.Markdown(`
					Maximum execution time before a task is killed, does not include
					artifact upload or image download time, etc.
				`),
			},
		},
		"perTaskLimit": schematypes.StringEnum{
			MetaData: schematypes.MetaData{
				Title: "Per Task Limits",
				Description: util.Markdown(`
					This plugin can 'forbid', 'allow' or 'require' tasks to specify
					'task.payload.maxRunTime', which if present must be less than
					'maxRunTime' as configured at plugin level.
				`),
			},
			Options: []string{
				limitRequire,
				limitAllow,
				limitForbid,
			},
		},
	},
	Required: []string{"maxRunTime", "perTaskLimit"},
}
