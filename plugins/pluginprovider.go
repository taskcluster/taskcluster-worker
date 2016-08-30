package plugins

import (
	"fmt"
	"sync"

	"github.com/Sirupsen/logrus"
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

var (
	mPlugins = sync.Mutex{}
	plugins  = make(map[string]PluginProvider)
)

// PluginOptions is a wrapper for the arguments/options given when instantiating
// a Plugin using PluginProvider.
//
// We wrap all arguments so that we can add additional properties without
// breaking source compatibility with older plugins.
type PluginOptions struct {
	Environment *runtime.Environment
	Engine      engines.Engine
	Log         *logrus.Entry
	Config      interface{}
}

// The PluginProvider interface must be implemented and registered by anyone
// implementing a Plugin.
//
// If an implementor can determine that a plugin isn't available at compile-time
// it is preferred not to register the plugin.
type PluginProvider interface {
	NewPlugin(options PluginOptions) (Plugin, error)

	// ConfigSchema returns schema for the PluginOptions.Config property.
	// May return nil, if no configuration should be given.
	ConfigSchema() schematypes.Schema
}

// PluginProviderBase is a base struct that provides empty implementations of
// some methods for PluginProvider
//
// Implementors of PluginProvider should embed this struct to ensure forward
// compatibility when we add new optional method to PluginProvider.
type PluginProviderBase struct{}

// ConfigSchema returns a nil schema.
func (PluginProviderBase) ConfigSchema() schematypes.Schema {
	return nil
}

// RegisterPlugin will register a PluginProvider.
// This is meant to be called from init(), and will panic if name is already
// used by another plugin.
func RegisterPlugin(name string, provider PluginProvider) {
	mPlugins.Lock()
	defer mPlugins.Unlock()

	if name == "disabled" {
		panic("Plugin name 'disabled' is reserved")
	}
	if _, ok := plugins[name]; ok {
		panic(fmt.Sprintf("Plugin name '%s' is already in use", name))
	}
	plugins[name] = provider
}

// Plugins returns map from plugin name to PluginProviders.
func Plugins() map[string]PluginProvider {
	mPlugins.Lock()
	defer mPlugins.Unlock()

	p := make(map[string]PluginProvider)
	for name, provider := range plugins {
		p[name] = provider
	}
	return p
}
