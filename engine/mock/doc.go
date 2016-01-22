// Package mock implements a MockEngine that doesn't really do anything, but
// allows us to test plugins without having to run a real engine.
package mock

import (
	"github.com/taskcluster/taskcluster-worker/engine"
	"github.com/taskcluster/taskcluster-worker/engine/extpoints"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

func init() {
	// Register the mock engine as an import side-effect
	extpoints.RegisterExtension(&mockEngineProvider{}, "mock")
}

type mockEngineProvider struct{}

// NewMockEngine create a mock engine.
func (*mockEngineProvider) NewEngine(
	context *runtime.EngineContext,
) engine.Engine {
	return nil
}

//  import "testing"
//
//  func TestNewEngine(t *testing.T) {
//  	t.Log(NewEngine("Mock"))
//  }
