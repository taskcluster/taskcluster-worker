package watchdog

import (
	"strconv"
	"time"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

type config struct {
	Timeout time.Duration `json:"timeout"`
}

var configSchema = schematypes.Object{
	Title: "Watchdog Plugin",
	Description: util.Markdown(`
		The watchdog plugin resets a timer whenever the worker is reported as
		idle or processes a step in a task. This ensure that the task-processing
		loop remains alive. If the timeout is exceeded, the watchdog will report
		to sentry and shutdown the worker immediately.

		This plugin is mainly useful to avoid stale workers cut in some livelock.
		Note: This plugin won't cause a timeout between 'Started()' and
		'Stopped()', as this would limit task run time, for this purpose use the
		'maxruntime' plugin.
	`),
	Properties: schematypes.Properties{
		"timeout": schematypes.Duration{
			Title: "Watchdog Timeout",
			Description: util.Markdown(`
				Timeout after which to kill the worker, timeout is reset whenever a
				task progresses, worker is reported idle or task is between
				'Started()' and 'Stopped()'.

				Defaults to ` + strconv.Itoa(defaultTimeout) + ` minutes, if not
				specified (or zero).

				This property is specified in seconds as integer or as string on the
				form '1 day 2 hours 3 minutes'.
			`),
			AllowNegative: false,
		},
	},
}
