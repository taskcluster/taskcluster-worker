package nativeengine

import "github.com/taskcluster/taskcluster-worker/engines"

type sandboxBuilder struct {
	engines.SandboxBuilderBase
	engine *engine
}

func (b *sandboxBuilder) StartSandbox() (engines.Sandbox, error) {
	return newSandbox(b)
}
