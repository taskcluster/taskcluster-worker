// Package shellclient provides a wrapper for demuxing a shell websocket and
// exposing the stdout/stderr streams as well as offering a way to provide the
// stdin stream.
package shellclient

import "github.com/taskcluster/taskcluster-worker/runtime"

var debug = runtime.Debug("shellclient")
