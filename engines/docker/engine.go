package dockerengine

import (
	docker "github.com/fsouza/go-dockerclient"
	"github.com/pkg/errors"
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/docker/network"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/caching"
)

type engine struct {
	engines.EngineBase
	Environment *runtime.Environment
	docker      *docker.Client
	monitor     runtime.Monitor
	config      configType
	cache       *caching.Cache
	networks    *network.Pool
}

type engineProvider struct {
	engines.EngineProviderBase
}

func init() {
	engines.Register("docker", engineProvider{})
}

func (p engineProvider) ConfigSchema() schematypes.Schema {
	return configSchema
}

func (p engineProvider) NewEngine(options engines.EngineOptions) (engines.Engine, error) {
	debug("docker engineProvider.NewEngine()")
	var c configType
	schematypes.MustValidateAndMap(configSchema, options.Config, &c)

	if c.DockerSocket == "" {
		c.DockerSocket = "unix:///var/run/docker.sock" // default docker socket
	}

	// Create docker client
	client, err := docker.NewClient(c.DockerSocket)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to docker socket at: %s", c.DockerSocket)
	}

	return &engine{
		config:      c,
		docker:      client,
		Environment: options.Environment,
		monitor:     options.Monitor,
		cache:       caching.New(imageConstructor, true, options.Environment.GarbageCollector, options.Monitor),
		networks:    network.NewPool(client, options.Monitor.WithPrefix("network-pool")),
	}, nil
}

type payloadType struct {
	Image   imageType `json:"image"`
	Command []string  `json:"command"`
}

var payloadSchema = schematypes.Object{
	Properties: schematypes.Properties{
		"image": imageSchema,
		"command": schematypes.Array{
			Title:       "Command",
			Description: "Command to run inside the container.",
			Items:       schematypes.String{},
		},
	},
	Required: []string{
		"image",
		"command",
	},
}

func (e *engine) PayloadSchema() schematypes.Object {
	return payloadSchema
}

func (e *engine) NewSandboxBuilder(options engines.SandboxOptions) (engines.SandboxBuilder, error) {
	var p payloadType
	schematypes.MustValidateAndMap(payloadSchema, options.Payload, &p)

	return newSandboxBuilder(&p, e, e.Environment.Monitor, options.TaskContext), nil
}

func (e *engine) Dispose() error {
	// Dispose network.Pool
	err := e.networks.Dispose()
	if err != nil {
		return errors.Wrap(err, "failed to dispose network.Pool")
	}

	return nil
}
