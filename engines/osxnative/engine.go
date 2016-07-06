//go:generate go-composite-schema --unexported --required engine payload-schema.yml generated_payloadschema.go

// Package osxnative implements the Mac OSX engine
package osxnative

import (
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/extpoints"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type engine struct {
	engines.EngineBase
	log *logrus.Entry
}

type engineProvider struct {
	extpoints.EngineProviderBase
}

func (e engineProvider) NewEngine(options extpoints.EngineOptions) (engines.Engine, error) {
	return engine{log: options.Log}, nil
}

func (e engineProvider) ConfigSchema() runtime.CompositeSchema {
	return runtime.NewEmptyCompositeSchema()
}

func (e engine) PayloadSchema() runtime.CompositeSchema {
	return payloadSchema
}

func (e engine) NewSandboxBuilder(options engines.SandboxOptions) (engines.SandboxBuilder, error) {
	taskPayload, ok := options.Payload.(*payload)
	if !ok {
		e.log.WithFields(logrus.Fields{
			"payload": options.Payload,
		}).Error("Invalid payload schema")

		return nil, engines.NewMalformedPayloadError("Invalid payload schema")
	}

	var m sync.Mutex
	return sandboxbuilder{
		SandboxBuilderBase: engines.SandboxBuilderBase{},
		env:                map[string]string{},
		taskPayload:        taskPayload,
		context:            options.TaskContext,
		envMutex:           &m,
	}, nil
}
