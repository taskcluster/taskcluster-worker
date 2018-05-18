// Package relengapi provides a taskcluster-worker plugin that exposes a proxy
// that foward requests to relengapi.
package relengapi

import "github.com/taskcluster/taskcluster-worker/runtime/util"

var debug = util.Debug("relengapi")
