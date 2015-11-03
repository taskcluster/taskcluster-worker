package mock

import (
	"github.com/taskcluster/taskcluster-worker/engine"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

// NewMockEngine create a mock engine.
func NewMockEngine(context *runtime.EngineContext) engine.Engine {
	return nil
}
