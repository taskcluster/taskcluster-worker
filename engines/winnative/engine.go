//go:generate go-composite-schema --unexported --required config config-schema.yml generated_configschema.go
//go:generate go-composite-schema --unexported --required payload payload-schema.yml generated_payloadschema.go

// Package winnative implements a worker engine that runs tasks on Windows™.
// The engine will create a new Windows™ (non-admin) user account for each task
// that runs.
package winnative

import (
	"github.com/Sirupsen/logrus"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/extpoints"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type engine struct {
	engines.EngineBase
	Log *logrus.Entry
}

type engineProvider struct{}

func init() {
	// Register the mock engine as an import side-effect
	extpoints.EngineProviders.Register(engineProvider{}, "winnative")
}

func (e engineProvider) NewEngine(options extpoints.EngineOptions) (engines.Engine, error) {
	return engine{Log: options.Log}, nil
}

func (e engineProvider) ConfigSchema() runtime.CompositeSchema {
	return configSchema
}

func (e engine) PayloadSchema() runtime.CompositeSchema {
	return payloadSchema
}

func (e engine) NewSandboxBuilder(options engines.SandboxOptions) (engines.SandboxBuilder, error) {
	e.Log.Debug("Building winnative Sandbox")
	p, valid := options.Payload.(*payload)
	if !valid {
		// TODO: Write to some sort of log if the type assertion fails
		return nil, engines.ErrContractViolation
	}
	return &sandboxBuilder{
		payload: p,
		context: options.TaskContext,
	}, nil
}
