// Package success implements a very simple plugin that looks that the
// ResultSet.Success() value to determine if the process from the sandbox
// exited successfully.
//
// Most engines implements ResultSet.Success() to mean the sub-process exited
// non-zero. In this plugin we use this in the Stopped() hook to ensure that
// tasks are declared "failed" if they had a non-zero exit code.
//
// The attentive reader might think this is remarkably simple and stupid plugin.
// This is true, but it does display the concept of plugins and more importantly
// removes a special case that we would otherwise have to take into
// consideration in the runtime.
package success

import (
	"github.com/taskcluster/taskcluster-worker/engine"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type successPlugin struct{}
type successFactory struct {
	plugins.PluginBase
}

// NewPluginFactory returns a new SuccessPluginFactory which creates plugins
// that checks the ResultSet for Success().
func NewPluginFactory(engine.Engine, *runtime.EngineContext) plugins.PluginFactory {
	return successFactory{}
}

func (successFactory) NewPlugin(*runtime.SandboxContextBuilder) Plugin {
	return successPlugin{}
}

func (successPlugin) Stopped(result engine.ResultSet) (bool, error) {
	// Here we return false resulting in the task being declared "failed", unless
	// the sandbox had a successful exit code. In practice this means the process
	// inside the sandbox exited non-zero.
	return result.Success(), nil
}
