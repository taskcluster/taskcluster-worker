// Package scriptengine provides an engine that can be configured with a script
// and a JSON schema, such that the worker executes declarative tasks.
package scriptengine

import "github.com/taskcluster/taskcluster-worker/runtime"

var debug = runtime.Debug("scriptengine")
