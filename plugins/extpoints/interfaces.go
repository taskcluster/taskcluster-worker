package extpoints

import (
	"sync"

	"github.com/taskcluster/taskcluster-worker/engines"
	p "github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

// The Plugin Factory is responsible for maintaining a list of plugin factories
// that are used to create new instances of a Plugin for a task execution.
type PluginFactory func(options *p.PluginOptions) p.Plugin

// NewPluginManager creates a manager for plugins used during the task execution.
// An instance of each registered plugin will be created and added to the manager.
func NewPluginManager(engine *engines.Engine, environment *runtime.Environment, task *runtime.TaskContext) *p.PluginManager {
	options := &p.PluginOptions{task}
	var plugins []p.Plugin

	// Wait group, so we can wait for all plugins to finish
	var wg sync.WaitGroup
	wg.Add(len(PluginFactories.All()))

	for _, factory := range PluginFactories.All() {
		factory = factory
		go func() {
			// TODO (garndt): add error handling if a plugin can't be initialized
			plugins = append(plugins, factory(options))
			wg.Done()
		}()
	}

	// Wait for plugins to be create and return a plugin wrapper
	wg.Wait()
	manager := &p.PluginManager{
		Plugins: plugins,
	}

	return manager
}
