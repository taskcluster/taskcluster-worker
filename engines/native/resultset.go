package nativeengine

import "github.com/taskcluster/taskcluster-worker/engines"

type resultSet struct {
	engines.ResultSetBase
	engine *engine
}

func (r *resultSet) Success() bool {
	return true
}
