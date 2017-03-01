package scriptengine

import (
	"fmt"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type engineProvider struct {
	engines.EngineProviderBase
}

type engine struct {
	engines.EngineBase
	monitor     runtime.Monitor
	config      configType
	schema      schematypes.Object
	environment *runtime.Environment
}

func init() {
	engines.Register("script", engineProvider{})
}

func (engineProvider) ConfigSchema() schematypes.Schema {
	return configSchema
}

func (engineProvider) NewEngine(options engines.EngineOptions) (engines.Engine, error) {
	var config configType
	if schematypes.MustMap(configSchema, options.Config, &config) != nil {
		return nil, engines.ErrContractViolation
	}
	// Construct payload schema as schematypes.Object using schema.properties
	properties := schematypes.Properties{}
	for k, s := range config.Schema.Properties {
		schema, err := schematypes.NewSchema(s)
		if err != nil {
			return nil, fmt.Errorf("Error loading schema: %s", err)
		}
		properties[k] = schema
	}

	return &engine{
		monitor: options.Monitor,
		config:  config,
		schema: schematypes.Object{
			Properties: properties,
		},
		environment: options.Environment,
	}, nil
}

func (e *engine) PayloadSchema() schematypes.Object {
	return e.schema
}

func (e *engine) NewSandboxBuilder(options engines.SandboxOptions) (engines.SandboxBuilder, error) {
	if e.schema.Validate(options.Payload) != nil {
		return nil, engines.ErrContractViolation
	}
	return &sandboxBuilder{
		payload: options.Payload,
		engine:  e,
		context: options.TaskContext,
		monitor: options.Monitor,
	}, nil
}
