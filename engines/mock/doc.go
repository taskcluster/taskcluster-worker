// Package mock implements a MockEngine that doesn't really do anything, but
// allows us to test plugins without having to run a real engine.
package mock

import (
	"errors"

	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/extpoints"
)

func init() {
	// Register the mock engine as an import side-effect
	extpoints.EngineProviders.Register(newMockEngine, "mock")
}

func newMockEngine(options extpoints.EngineOptions) (engines.Engine, error) {
	return nil, errors.New("Engine not yet implemented")
}
