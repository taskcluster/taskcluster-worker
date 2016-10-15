package nativeengine

import "github.com/taskcluster/taskcluster-worker/engines"

type engineProvider struct {
	engines.EngineProviderBase
}

func init() {
	engines.Register("native", engineProvider{})
}

func (engineProvider) NewEngine(options engines.EngineOptions) (engines.Engine, error) {
	return &engine{}, nil
}

type engine struct {
	engines.EngineBase
}

func (e *engine) NewSandboxBuilder(options engines.SandboxOptions) (engines.SandboxBuilder, error) {
	return &sandboxBuilder{engine: e}, nil
}
