package dockerengine

import (
	"fmt"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/pkg/errors"
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/docker/network"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/caching"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
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
	Image      imageType `json:"image"`
	Command    []string  `json:"command"`
	Privileged bool      `json:"privileged"`
}

func (e *engine) PayloadSchema() schematypes.Object {
	payloadSchema := schematypes.Object{
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

	// If we allow running in privileged mode, we also need a task.payload
	// property to indicated it.
	if e.config.Privileged == privilegedAllow {
		payloadSchema.Properties["privileged"] = schematypes.Boolean{
			Title: "Privileged",
			Description: util.Markdown(`
				Run the task docker container in privileged mode.

				Setting this option requires that 'task.scopes' contains the scope
				'worker:privileged:<provisionerId>/<workerType>'.
			`),
		}
	}

	return payloadSchema
}

func (e *engine) NewSandboxBuilder(options engines.SandboxOptions) (engines.SandboxBuilder, error) {
	var p payloadType
	schematypes.MustValidateAndMap(e.PayloadSchema(), options.Payload, &p)

	// Check if privileged == true is allowed
	switch e.config.Privileged {
	case privilegedAllow: // Check scope if p.Privileged is true
		if p.Privileged {
			scope := fmt.Sprintf("worker:privileged:%s/%s", e.Environment.ProvisionerID, e.Environment.WorkerType)
			if !options.TaskContext.HasScopes([]string{scope}) {
				return nil, runtime.NewMalformedPayloadError(fmt.Sprintf(
					"'task.payload.privileged' is 'true', but this worker requires 'task.scopes' to grant the scope: '%s' "+
						"in order for task containers to run in privileged mode.",
					scope,
				))
			}
		}
	case privilegedNever: // In this case p.Privileged must be false
		if p.Privileged {
			panic(errors.New("config has privileged: 'never', but payload.privileged = true happened"))
		}
	case privilegedAlways: // Just force p.Privileged = true
		p.Privileged = true
	}

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
