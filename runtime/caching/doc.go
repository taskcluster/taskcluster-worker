// Package caching provides an easy to make a cache on top of the gc package
// used to track idle resources in taskcluster-worker.
package caching

import "github.com/taskcluster/taskcluster-worker/runtime/util"

var debug = util.Debug("caching")
