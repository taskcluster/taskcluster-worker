// Package logprefix provides a taskcluster-worker plugin that prefixes all
// task logs with useful debug information such as taskId, workerType, as well
// as configurable constants.
package logprefix

import "github.com/taskcluster/taskcluster-worker/runtime/util"

var debug = util.Debug("logprefix")
