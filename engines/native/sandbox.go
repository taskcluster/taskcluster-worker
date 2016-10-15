package nativeengine

import "github.com/taskcluster/taskcluster-worker/engines"

type sandbox struct {
	engines.SandboxBase
	engine *engine
}

func newSandbox(b *sandboxBuilder) (*sandbox, error) {
	return nil, nil
}

func (s *sandbox) WaitForResult() (engines.ResultSet, error) {
	return &resultSet{engine: s.engine}, nil
}
