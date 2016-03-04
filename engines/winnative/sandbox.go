package winnative

import (
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type sandboxBuilder struct {
	engines.SandboxBuilderBase
	payload *payload
	context *runtime.TaskContext
}

type sandbox struct {
	engines.SandboxBase
}

type resultSet struct {
	engines.ResultSetBase
}

func (s *sandboxBuilder) StartSandbox() (engines.Sandbox, error) {
	return new(sandbox), nil
}

func (s *sandbox) WaitForResult() (engines.ResultSet, error) {
	return new(resultSet), nil
}

func (r *resultSet) Success() bool {
	return true
}
