// Package tasklog provides a taskcluster-worker plugin that uploads a static
// task.log when the task is finished.
//
// This plugin should not be used in combination with the 'livelog' plugin, but
// is instead an alternative to the 'livelog' plugin. This plugin provides a
// strict subset of features offered by the 'livelog' plugin, however, this
// plugin will not offer any interactive aspects, hence, some might consider it
// more secure.
package tasklog

import "github.com/taskcluster/taskcluster-worker/runtime/util"

var debug = util.Debug("tasklog")
