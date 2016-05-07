//go:generate go-composite-schema --unexported --required start payload-schema.yml generated_payloadschema.go
//go:generate go-composite-schema --unexported --required qemu config-schema.yml generated_configschema.go

package qemuengine

import (
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/extpoints"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/image"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/network"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type engine struct {
	engines.EngineBase
	Log          *logrus.Entry
	imageManager *image.Manager
	networkPool  *network.Pool
}

type engineProvider struct {
	extpoints.EngineProviderBase
}

func init() {
	extpoints.EngineProviders.Register(engineProvider{}, "qemu")
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
		Log:          options.Log,
		imageManager: imageManager,
		networkPool:  networkPool,
	}, nil
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

	fmt.Println(p.Image)
	return nil, nil
}
