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
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/plugins/extpoints"
)

func init() {
	extpoints.PluginFactories.Register(NewSuccessPlugin, "success")
}

func NewSuccessPlugin(options *plugins.PluginOptions) plugins.Plugin {
	return Success{}
}

type Success struct {
	plugins.PluginBase
}
