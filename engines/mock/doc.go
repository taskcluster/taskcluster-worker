// Package mockengine implements a MockEngine that doesn't really do anything,
// but allows us to test plugins without having to run a real engine.
package mockengine

import "github.com/taskcluster/taskcluster-worker/runtime/util"

var debug = util.Debug("mockengine")
