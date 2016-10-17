package nativeengine

import (
	"github.com/Sirupsen/logrus"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type resultSet struct {
	engines.ResultSetBase
	engine  *engine
	context *runtime.TaskContext
	log     *logrus.Entry
}

func (r *resultSet) Success() bool {
	return true
}
