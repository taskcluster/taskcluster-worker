// Package version provides a CommandProvider that displays version number and
// git revision, these values are also exported through methods so that they
// can be read from other packages.
package version

import "github.com/taskcluster/taskcluster-worker/runtime/util"

var debug = util.Debug("version")
