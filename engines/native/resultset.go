package nativeengine

import (
	"github.com/Sirupsen/logrus"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/native/system"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

type resultSet struct {
	engines.ResultSetBase
	engine     *engine
	context    *runtime.TaskContext
	log        *logrus.Entry
	homeFolder runtime.TemporaryFolder
	user       *system.User
	success    bool
}

func (r *resultSet) Success() bool {
	return r.success
}

func (r *resultSet) ExtractFile(path string) (ioext.ReadSeekCloser, error) {
	return nil, nil
}

func (r *resultSet) ExtractFolder(path string, handler engines.FileHandler) error {
	return nil
}

func (r *resultSet) Dispose() error {
	return nil
}
