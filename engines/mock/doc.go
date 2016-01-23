// Package mock implements a MockEngine that doesn't really do anything, but
// allows us to test plugins without having to run a real engine.
package mock

import (
	"errors"

	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/extpoints"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

func init() {
	// Register the mock engine as an import side-effect
	extpoints.EngineProviders.Register(NewEngine, "mock")
}

func NewEngine(environment *runtime.Environment, options *engines.EngineOptions) (engines.Engine, error) {
	return nil, errors.New("Engine not yet implemented")
}
