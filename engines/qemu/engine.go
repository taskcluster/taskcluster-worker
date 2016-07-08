//go:generate go-composite-schema --unexported --required start payload-schema.yml generated_payloadschema.go
//go:generate go-composite-schema --unexported --required qemu config-schema.yml generated_configschema.go

package qemuengine

import (
	"github.com/Sirupsen/logrus"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/extpoints"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/image"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/network"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type engine struct {
	engines.EngineBase
	engineConfig *engineConfig
	Log          *logrus.Entry
	imageManager *image.Manager
	networkPool  *network.Pool
	Environment  *runtime.Environment
}

type engineProvider struct {
	extpoints.EngineProviderBase
}

func (p engineProvider) ConfigSchema() runtime.CompositeSchema {
	return engineConfigSchema
}

func (p engineProvider) NewEngine(options extpoints.EngineOptions) (engines.Engine, error) {
	// Cast config to engineConfig
	c, ok := options.Config.(*engineConfig)
	if !ok {
		return nil, engines.ErrContractViolation
	}

	// Create image manager
	imageManager, err := image.NewManager(
		c.ImageFolder,
		options.Environment.GarbageCollector,
		options.Log.WithField("subsystem", "image-manager"),
		options.Environment.Sentry,
	)
	if err != nil {
		return nil, err
	}

	// Create network pool
	networkPool, err := network.NewPool(c.MaxConcurrency)
	if err != nil {
		return nil, err
	}

	// Construct engine object
	return &engine{
		engineConfig: c,
		Log:          options.Log,
		imageManager: imageManager,
		networkPool:  networkPool,
		Environment:  options.Environment,
	}, nil
}

func (e *engine) Capabilities() engines.Capabilities {
	return engines.Capabilities{
		MaxConcurrency: e.engineConfig.MaxConcurrency,
	}
}

func (e *engine) PayloadSchema() runtime.CompositeSchema {
	return qemuPayloadSchema
}

func (e *engine) NewSandboxBuilder(options engines.SandboxOptions) (engines.SandboxBuilder, error) {
	// Cast payload to qemuPayload
	p, ok := options.Payload.(*qemuPayload)
	if !ok {
		return nil, engines.ErrContractViolation
	}

	// Get an idle network
	net, err := e.networkPool.Network()
	if err == network.ErrAllNetworksInUse {
		return nil, engines.ErrMaxConcurrencyExceeded
	}
	if err != nil {
		return nil, err
	}

	// Create sandboxBuilder, it'll handle image downloading
	return newSandboxBuilder(p, net, options.TaskContext, e), nil
}

func (e *engine) Dispose() error {
	err := e.networkPool.Dispose()
	e.networkPool = nil
	return err
}
