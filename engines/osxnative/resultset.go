package osxnative

import (
	"github.com/taskcluster/taskcluster-worker/engines"
)

type resultset struct {
	engines.ResultSetBase
	success bool
}

func (r resultset) Success() bool {
	return r.success
}
