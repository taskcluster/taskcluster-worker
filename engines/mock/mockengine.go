// Package mockengine implements a MockEngine that doesn't really do anything,
// but allows us to test plugins without having to run a real engine.
package mockengine

import (
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/extpoints"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type engine struct {
	engines.EngineBase
}

func init() {
	// Register the mock engine as an import side-effect
	extpoints.EngineProviders.Register(func(
		options extpoints.EngineOptions,
	) (engines.Engine, error) {
		return engine{}, nil
	}, "mock")
}

func (engine) PayloadSchema() runtime.CompositeSchema {
	return runtime.NewEmptyCompositeSchema()
}

func (engine) NewSandboxBuilder(options engines.SandboxOptions) (engines.SandboxBuilder, error) {
	return &sandbox{}, nil
}

// NewCacheFolder returns ErrFeatureNotSupported indicating that the feature
// isn't supported.
func (engine) NewCacheFolder() (engines.Volume, error) {
	return nil, engines.ErrFeatureNotSupported
}
