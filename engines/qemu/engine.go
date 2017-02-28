package qemuengine

import (
	"github.com/Sirupsen/logrus"
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/image"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/network"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/vm"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type engine struct {
	engines.EngineBase
	engineConfig configType
	Log          *logrus.Entry
	imageManager *image.Manager
	networkPool  *network.Pool
	Environment  *runtime.Environment
}

type engineProvider struct {
	engines.EngineProviderBase
}

type configType struct {
	MaxConcurrency int               `json:"maxConcurrency"`
	ImageFolder    string            `json:"imageFolder"`
	SocketFolder   string            `json:"socketFolder"`
	MachineOptions vm.MachineOptions `json:"machineOptions"`
}

var configSchema = schematypes.Object{
	Properties: schematypes.Properties{
		"maxConcurrency": schematypes.Integer{
			MetaData: schematypes.MetaData{
				Title:       "Max Concurrency",
				Description: `Maximum number of virtual machines to run concurrently.`,
			},
			Minimum: 1,
			Maximum: 64,
		},
		"imageFolder": schematypes.String{
			MetaData: schematypes.MetaData{
				Title: "Image Folder",
				Description: `Path to folder to be used for image storage and cache.
											Please ensure this has lots of space.`,
			},
		},
		"socketFolder": schematypes.String{
			MetaData: schematypes.MetaData{
				Title: "Socket Folder",
				Description: `Path to folder to be used for internal unix-domain sockets.
											Ideally, this shouldn't be readable by anyone else.`,
			},
		},
		"machineOptions": vm.MachineOptionsSchema,
	},
	Required: []string{
		"imageFolder",
		"maxConcurrency",
		"socketFolder",
		"machineOptions",
	},
}

func (p engineProvider) ConfigSchema() schematypes.Schema {
	return configSchema
}

func (p engineProvider) NewEngine(options engines.EngineOptions) (engines.Engine, error) {
	var c configType
	err := configSchema.Map(options.Config, &c)
	if err == schematypes.ErrTypeMismatch {
		panic("Type mismatch")
	} else if err != nil {
		return nil, engines.ErrContractViolation
	}

	// Create image manager
	imageManager, err := image.NewManager(
		c.ImageFolder,
		options.Environment.GarbageCollector,
		options.Log.WithField("subsystem", "image-manager"),
		options.Environment.Monitor.WithPrefix("image-manager"),
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

type payloadType struct {
	Image   string   `json:"image"`
	Command []string `json:"command"`
}

var payloadSchema = schematypes.Object{
	Properties: schematypes.Properties{
		"image": schematypes.URI{
			MetaData: schematypes.MetaData{
				Title: "Image to download",
				Description: "URL to an image file. This is a zstd compressed " +
					"tar-archive containing a raw disk image `disk.img`, a qcow2 " +
					"overlay `layer.qcow2` and a machine definition file " +
					"`machine.json`. Refer to engine documentation for more details.",
			},
		},
		"command": schematypes.Array{
			MetaData: schematypes.MetaData{
				Title:       "Command to run",
				Description: `Command and arguments to execute on the guest.`,
			},
			Items: schematypes.String{},
		},
	},
	Required: []string{"command", "image"},
}

func (e *engine) PayloadSchema() schematypes.Object {
	return payloadSchema
}

func (e *engine) NewSandboxBuilder(options engines.SandboxOptions) (engines.SandboxBuilder, error) {
	var p payloadType
	if schematypes.MustMap(payloadSchema, options.Payload, &p) != nil {
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
	return newSandboxBuilder(&p, net, options.TaskContext, e), nil
}

func (e *engine) Dispose() error {
	err := e.networkPool.Dispose()
	e.networkPool = nil
	return err
}
