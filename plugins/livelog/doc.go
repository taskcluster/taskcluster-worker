// Package livelog provides a taskcluster-worker plugin that makes the task log
// available as a live log during task execution and finally uploads it as a
// static log.
package livelog

import "github.com/taskcluster/taskcluster-worker/runtime/util"

var debug = util.Debug("livelog")
