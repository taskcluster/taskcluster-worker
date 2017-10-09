package mockengine

import (
	"net/http"
	"time"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type engine struct {
	engines.EngineBase
	monitor     runtime.Monitor
	environment runtime.Environment
}

type engineProvider struct {
	engines.EngineProviderBase
}

func init() {
	// Register the mock engine as an import side-effect
	engines.Register("mock", engineProvider{})
}

// New creates a new MockEngine
func New(options engines.EngineOptions) engines.Engine {
	engine, _ := engineProvider{}.NewEngine(options)
	return engine
}

func (e engineProvider) NewEngine(options engines.EngineOptions) (engines.Engine, error) {
	if options.Environment.Monitor == nil {
		panic("EngineOptions.Environment.Monitor is nil, this is a contract violation")
	}
	if options.Monitor == nil {
		panic("EngineOptions.Monitor is nil, this is a contract violation")
	}
	return engine{
		monitor:     options.Monitor,
		environment: *options.Environment,
	}, nil
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
	if p.Function == "malformed-payload-initial" {
		<-time.After(time.Duration(p.Delay) * time.Millisecond)
		return nil, runtime.NewMalformedPayloadError(p.Argument)
	}
	return &sandbox{
		environment: e.environment,
		payload:     p,
		context:     options.TaskContext,
		mounts:      make(map[string]*mount),
		proxies:     make(map[string]http.Handler),
		env:         make(map[string]string),
		files:       make(map[string][]byte),
	}, nil
}

func (engine) VolumeSchema() schematypes.Schema {
	return schematypes.Object{}
}

func (engine) NewVolumeBuilder(options interface{}) (engines.VolumeBuilder, error) {
	// Create a new cache folder
	return &volume{
		files: make(map[string]string),
	}, nil
}

func (engine) NewVolume(options interface{}) (engines.Volume, error) {
	// Create a new cache folder
	return &volume{
		files: make(map[string]string),
	}, nil
}
