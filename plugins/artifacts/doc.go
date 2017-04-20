// Package artifacts is provides a taskcluster-worker plugin that uploads
// artifacts when sandbox execution has stopped.
package artifacts

import "github.com/taskcluster/taskcluster-worker/runtime/util"

var debug = util.Debug("artifacts")
