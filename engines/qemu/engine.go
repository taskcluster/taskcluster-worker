package qemuengine

import (
	"github.com/pkg/errors"
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/image"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/network"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/vm"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type engine struct {
	engines.EngineBase
	engineConfig   configType
	defaultMachine vm.Machine
	monitor        runtime.Monitor
	imageManager   *image.Manager
	networkPool    *network.Pool
	Environment    *runtime.Environment
	maxConcurrency int
	socketFolder   runtime.TemporaryFolder
}

type engineProvider struct {
	engines.EngineProviderBase
}

type configType struct {
	Network       interface{}      `json:"network"`
	MachineLimits vm.MachineLimits `json:"limits"`
	Machine       interface{}      `json:"machine"`
}

var configSchema = schematypes.Object{
	Properties: schematypes.Properties{
		"network": network.PoolConfigSchema,
		"limits":  vm.MachineLimitsSchema,
		"machine": vm.MachineSchema,
	},
	Required: []string{
		"network",
		"limits",
	},
}

func (p engineProvider) ConfigSchema() schematypes.Schema {
	return configSchema
}

func (p engineProvider) NewEngine(options engines.EngineOptions) (engines.Engine, error) {
	var c configType
	schematypes.MustValidateAndMap(configSchema, options.Config, &c)

	// Create socket folder
	socketFolder, err := options.Environment.TemporaryStorage.NewFolder()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create socket folder")
	}

	// Create image manager
	imageManager, err := image.NewManager(
		options.Environment.TemporaryStorage.NewFilePath(),
		options.Environment.GarbageCollector,
		options.Environment.Monitor.WithPrefix("image-manager"),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create image manager")
	}

	// Create network pool
	networkPool, err := network.NewPool(network.PoolOptions{
		Config:           c.Network,
		Monitor:          options.Monitor.WithPrefix("network"),
		TemporaryStorage: options.Environment.TemporaryStorage,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create network pool")
	}

	// Create defaultMachine machine from config
	var defaultMachine vm.Machine
	if c.Machine != nil {
		defaultMachine = vm.NewMachine(c.Machine)
	}

	// Construct engine object
	return &engine{
		engineConfig:   c,
		defaultMachine: defaultMachine,
		monitor:        options.Monitor,
		imageManager:   imageManager,
		networkPool:    networkPool,
		maxConcurrency: networkPool.Size(),
		Environment:    options.Environment,
		socketFolder:   socketFolder,
	}, nil
}

func (e *engine) Capabilities() engines.Capabilities {
	return engines.Capabilities{
		MaxConcurrency: e.maxConcurrency,
	}
}

type payloadType struct {
	Image   interface{} `json:"image"`
	Command []string    `json:"command"`
	Machine interface{} `json:"machine,omitempty"`
}

var payloadSchema = schematypes.Object{
	Properties: schematypes.Properties{
		"image": imageFetcher.Schema(),
		"command": schematypes.Array{
			Title:       "Command to run",
			Description: `Command and arguments to execute on the guest.`,
			Items:       schematypes.String{},
		},
		"machine": vm.MachineSchema,
	},
	Required: []string{"command", "image"},
}

func (e *engine) PayloadSchema() schematypes.Object {
	return payloadSchema
}

func (e *engine) NewSandboxBuilder(options engines.SandboxOptions) (engines.SandboxBuilder, error) {
	var p payloadType
	schematypes.MustValidateAndMap(payloadSchema, options.Payload, &p)

	// Get an idle network
	net, err := e.networkPool.Network()
	if err == network.ErrAllNetworksInUse {
		return nil, engines.ErrMaxConcurrencyExceeded
	}
	if err != nil {
		return nil, err
	}

	// Create sandboxBuilder, it'll handle image downloading
	return newSandboxBuilder(&p, net, options.TaskContext, e, options.Monitor), nil
}

func (e *engine) Dispose() error {
	err := e.networkPool.Dispose()
	e.networkPool = nil
	return err
}
