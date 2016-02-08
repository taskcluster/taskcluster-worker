package extpoints

import (
	"github.com/Sirupsen/logrus"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

// PluginOptions is a wrapper for the arguments/options given when instantiating
// a Plugin using PluginProvider.
//
// We wrap all arguments so that we can add additional properties without
// breaking source compatibility with older plugins.
// Note: This is passed by-value for efficiency (and to prohibit nil), if
// adding any large fields please consider adding them as pointers.
// Note: This is intended to be a simple argument wrapper, do not add methods
// to this struct.
type PluginOptions struct {
	environment *runtime.Environment
	engine      *engines.Engine
	log         *logrus.Entry
}

// The PluginProvider interface must be implemented and registered by anyone
// implementing a Plugin.
//
// If an implementor can determine that a plugin isn't available at compile-time
// it is preferred not to register the plugin.
type PluginProvider func(options PluginOptions) (plugins.Plugin, error)
