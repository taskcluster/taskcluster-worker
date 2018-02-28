package dockerengine

import (
	"sync"

	docker "github.com/fsouza/go-dockerclient"
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/caching"
	"github.com/taskcluster/taskcluster-worker/runtime/gc"
)

type engine struct {
	engines.EngineBase
	m              sync.Mutex
	Environment    *runtime.Environment
	client         *docker.Client
	monitor        runtime.Monitor
	maxConcurrency int
	engineConfig   configType
	running        int
	cache          *caching.Cache
}

type engineProvider struct {
	engines.EngineProviderBase
	cache *caching.Cache
}

func init() {
	engines.Register("docker", engineProvider{
		cache: caching.New(imageConstructor, true, &gc.GarbageCollector{}),
	})
}

type configType struct {
	DockerEndpoint string `json:"dockerEndpoint"`
	MaxConcurrency int    `json:"maxConcurrency"`
}

var configSchema = schematypes.Object{
	Properties: schematypes.Properties{
		"dockerEndpoint": schematypes.String{
			Title: "Docker Endpoint",
			Description: "dockerEndpoint is the endpoint to use for communicating\n" +
				"with the Docker daemon.",
			//TODO: Add pattern for docker socket
		},
		"maxConcurrency": schematypes.Integer{
			Title: "Max Concurrency",
			Description: "maxConcurrency defines the maximum number of tasks \n" +
				"that may run concurrently on the worker.",
			Minimum: 0,
			Maximum: 10,
		},
	},
	Required: []string{
		"dockerEndpoint",
	},
}

func (p engineProvider) ConfigSchema() schematypes.Schema {
	return configSchema
}

func (p engineProvider) NewEngine(options engines.EngineOptions) (engines.Engine, error) {
	debug("docker engineProvider.NewEngine()")
	var c configType
	schematypes.MustValidateAndMap(configSchema, options.Config, &c)

	client, err := docker.NewClient(c.DockerEndpoint)
	if err != nil {
		return nil, err
	}

	return &engine{
		engineConfig:   c,
		client:         client,
		Environment:    options.Environment,
		monitor:        options.Monitor,
		maxConcurrency: c.MaxConcurrency,
		cache:          p.cache,
		running:        0,
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
	e.m.Lock()
	defer e.m.Unlock()
	if e.maxConcurrency == e.running {
		return nil, engines.ErrMaxConcurrencyExceeded
	}
	e.running += 1
	return newSandboxBuilder(&p, e, e.Environment.Monitor, options.TaskContext), nil
}

func (e *engine) Dispose() error {
	return nil
}
