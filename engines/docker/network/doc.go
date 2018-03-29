// Package network wraps docker network and ensures exposure of HTTP end-points
// to containers attached to said network.
package network

import "github.com/taskcluster/taskcluster-worker/runtime/util"

var debug = util.Debug("network")
