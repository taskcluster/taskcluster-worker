package nativeengine

import (
	"github.com/Sirupsen/logrus"
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type engineProvider struct {
	engines.EngineProviderBase
}

type engine struct {
	engines.EngineBase
	environment *runtime.Environment
	log         *logrus.Entry
}

func init() {
	engines.Register("native", engineProvider{})
}

func (engineProvider) NewEngine(options engines.EngineOptions) (engines.Engine, error) {
	return &engine{
		environment: options.Environment,
		log:         options.Log,
	}, nil
}

func (e *engine) PayloadSchema() schematypes.Object {
	return payloadSchema
}

func (e *engine) NewSandboxBuilder(options engines.SandboxOptions) (engines.SandboxBuilder, error) {
	var p payload
	if schematypes.MustMap(payloadSchema, options.Payload, &p) != nil {
		return nil, engines.ErrContractViolation
	}
	return &sandboxBuilder{
		engine:  e,
		payload: p,
		context: options.TaskContext,
		env:     make(map[string]string),
		log: e.log.
			WithField("taskId", options.TaskContext.TaskID).
			WithField("runId", options.TaskContext.RunID),
	}, nil
}
