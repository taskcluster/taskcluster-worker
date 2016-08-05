//go:generate go-composite-schema --unexported --required start payload-schema.yml generated_payloadschema.go

package mockengine

import (
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/extpoints"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type engine struct {
	engines.EngineBase
	Log *logrus.Entry
}

type engineProvider struct {
	extpoints.EngineProviderBase
}

func init() {
	// Register the mock engine as an import side-effect
	extpoints.EngineProviders.Register(engineProvider{}, "mock")
}

func (e engineProvider) NewEngine(options extpoints.EngineOptions) (engines.Engine, error) {
	return engine{Log: options.Log}, nil
}

// mock config contains no fields
func (e engineProvider) ConfigSchema() runtime.CompositeSchema {
	return runtime.NewEmptyCompositeSchema()
}

func (e engine) PayloadSchema() runtime.CompositeSchema {
	return payloadSchema
}

func (e engine) NewSandboxBuilder(options engines.SandboxOptions) (engines.SandboxBuilder, error) {
	// We know that payload was created with CompositeSchema.Parse() from the
	// schema returned by PayloadSchema(), so here we type assert that it is
	// indeed a pointer to such a thing.
	e.Log.Debug("Building Sandbox")
	p, valid := options.Payload.(*payload)
	if !valid {
		// TODO: Write to some sort of log if the type assertion fails
		return nil, engines.ErrContractViolation
	}
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
