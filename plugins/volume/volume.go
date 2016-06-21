//go:generate go-composite-schema --unexported volumes payload-schema.yml generated_payloadschema.go

// Package volume implements a plugin for managing various volume types supported
// by the taskcluster-worker engines.  At a higher level there is one main volume
// plugin that will issue new volumes for each task run.  One notable exception to this
// are persistent volumes where these volumes should be retained on the host between
// task runs.  In the case of a persistent volume, there is a volume manager with a list
// of name to host path mappings.
package volume

import (
	"github.com/Sirupsen/logrus"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/plugins/extpoints"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type volumeProvider struct {
	extpoints.PluginProviderBase
}

type volumeManager struct {
	plugins.PluginBase
	environment *runtime.Environment
	engine      engines.Engine
	log         *logrus.Entry
	plugins     map[string]volumePlugin
}

type volumePlugin interface {
	newVolume(*volumeOptions) (volume, error)
}

// NewPlugin returns a volume plugin that manages a list of lower level volume
// plugins
func (volumeProvider) NewPlugin(opts extpoints.PluginOptions) (plugins.Plugin, error) {
	plugins := make(map[string]volumePlugin)
	plugins["persistent"], _ = NewPersistentVolumePlugin(opts)
	vm := volumeManager{
		environment: opts.Environment,
		engine:      opts.Engine,
		log:         opts.Log,
		plugins:     plugins,
	}

	return vm, nil
}

func (volumeManager) PayloadSchema() (runtime.CompositeSchema, error) {
	return payloadSchema, nil
}

func (v volumeManager) NewTaskPlugin(options plugins.TaskPluginOptions) (plugins.TaskPlugin, error) {
	v.log.Debugf("Creating volume task plugins for %s", options.TaskInfo.TaskID)
	if options.Payload == nil {
		return plugins.TaskPluginBase{}, nil
	}
	vs := &volumeSet{
		TaskPluginBase: plugins.TaskPluginBase{},
		vm:             &v,
	}

	vs.payload = *(options.Payload.(*payload))
	return vs, nil
}

func init() {
	extpoints.PluginProviders.Register(new(volumeProvider), "volume")
}
