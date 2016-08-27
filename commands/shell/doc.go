// Package shell provides a CommandProvider that implements a CLI tool for
// opening to a interactive shell to an interactive taskcluster-worker task
// in your terminal.
package shell

import "github.com/taskcluster/taskcluster-worker/runtime"

var debug = runtime.Debug("shell")
