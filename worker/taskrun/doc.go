// Package taskrun provides abstractions to execute a run of a task given a
// task, engine, plugin, and other runtime objects required by plugin and engine.
//
// This package will report back the status that should be reported to
// the queue, but it will not undertake any interaction with the queue.
// Other than passing a queue object on to engines and plugins through the
// TaskContext.
package taskrun

import "github.com/taskcluster/taskcluster-worker/runtime/util"

var debug = util.Debug("taskrun")
