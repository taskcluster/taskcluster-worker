package mockengine

import (
	"net/http"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type engine struct {
	engines.EngineBase
	monitor runtime.Monitor
}

type engineProvider struct {
	engines.EngineProviderBase
}

func init() {
	// Register the mock engine as an import side-effect
	engines.Register("mock", engineProvider{})
}

func (e engineProvider) NewEngine(options engines.EngineOptions) (engines.Engine, error) {
	if options.Environment.Monitor == nil {
		panic("EngineOptions.Environment.Monitor is nil, this is a contract violation")
	}
	if options.Monitor == nil {
		panic("EngineOptions.Monitor is nil, this is a contract violation")
	}
	return engine{monitor: options.Monitor}, nil
}

// mock config contains no fields
func (e engineProvider) ConfigSchema() schematypes.Schema {
	return schematypes.Object{}
}

func (e engine) PayloadSchema() schematypes.Object {
	return payloadSchema
}

func (e engine) NewSandboxBuilder(options engines.SandboxOptions) (engines.SandboxBuilder, error) {
	e.monitor.Debug("Building Sandbox")

	// Some sanity checks to ensure that we're providing all the options
	if options.Monitor == nil {
		panic("SandboxOptions.Monitor is nil, this is a contract violation")
	}
	if options.TaskContext == nil {
		panic("SandboxOptions.TaskContext is nil, this is a constract violation")
	}

	var p payloadType
	schematypes.MustValidateAndMap(payloadSchema, options.Payload, &p)
	return &sandbox{
		payload: p,
		context: options.TaskContext,
		mounts:  make(map[string]*mount),
		proxies: make(map[string]http.Handler),
		env:     make(map[string]string),
		files:   make(map[string][]byte),
	}, nil
}

func (engine) NewCacheFolder() (engines.Volume, error) {
	// Create a new cache folder
	return &volume{}, nil
}
