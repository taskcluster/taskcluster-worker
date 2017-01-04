// Package fetcher provides means for plugins and engines to fetch resources
// with generic references. Hence, the format for reference is consistent across
// plugins and engines, and we can re-use the download logic.
package fetcher

import "github.com/taskcluster/taskcluster-worker/runtime/util"

var debug = util.Debug("fetcher")
