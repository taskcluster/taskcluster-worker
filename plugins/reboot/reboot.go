// Package reboot is a plugin that supports rebooting the host running the worker.
//
// In some cases, especially running without a very tight
// sandbox, this is desirable after specific tests.
//
// In other cases, reboots are useful after a configured uptime, to
// cycle the host's configuration.
//
// To reboot after tasks, users must add the boolean "reboot" payload
// attribute.
package reboot

import (
	"fmt"

	"sync"
	"time"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type plugin struct {
	plugins.PluginBase
	environment *runtime.Environment

	// the command to reboot the host
	rebootCommand []string

	// while this is held for reading, there are tasks running
	runningTasks sync.RWMutex
}

type payloadType struct {
	RebootWhenDone bool `json:"reboot"`
}

type config struct {
	RebootAfter   int      `json:"rebootAfter"`
	RebootCommand []string `json:"rebootCommand"`
}

type taskPlugin struct {
	plugins.TaskPluginBase
	plugin         *plugin
	rebootWhenDone bool
}

type pluginProvider struct {
	plugins.PluginProviderBase
}

var payloadSchema = schematypes.Object{
	Properties: schematypes.Properties{
		"reboot": schematypes.Boolean{
			MetaData: schematypes.MetaData{
				Title:       "Reboot machine",
				Description: "If true, reboot the machine after task is finished.",
			},
		},
	},
}

var configSchema = schematypes.Object{
	Properties: schematypes.Properties{
		"rebootAfter": schematypes.Integer{
			MetaData: schematypes.MetaData{
				Title: "Reboot After",
				Description: `When the worker is idle after this many hours, it will reboot.
					If zero, no idle reboot will occur.`,
			},
			Minimum: 0,
			Maximum: 10000,
		},
		"rebootCommand": schematypes.Array{
			MetaData: schematypes.MetaData{
				Title:       "Reboot Command",
				Description: `The command to use to reboot the host, as a list of strings`,
			},
			Items: schematypes.String{},
		},
	},
}

func init() {
	plugins.Register("reboot", pluginProvider{})
}

func (pluginProvider) ConfigSchema() schematypes.Schema {
	return configSchema
}

func (pluginProvider) NewPlugin(options plugins.PluginOptions) (plugins.Plugin, error) {
	var c config
	if err := schematypes.MustMap(configSchema, options.Config, &c); err != nil {
		return nil, fmt.Errorf("While reading reboot plugin config: %s", err)
	}

	plugin := &plugin{
		PluginBase:    plugins.PluginBase{},
		environment:   options.Environment,
		rebootCommand: c.RebootCommand,
		runningTasks:  sync.RWMutex{},
	}

	if c.RebootAfter > 0 {
		// After rebootAfter hours, call the reboot method. If the lock is held by one or more
		// running tasks, the reboot will not occur until they finish (but no further tasks will
		// be able to start)
		time.AfterFunc(time.Duration(c.RebootAfter)*time.Second, func() {
			plugin.environment.Monitor.Warn("Host rebootAfter has expired; scheduling reboot")
			plugin.reboot()
		})
	}

	return plugin, nil
}

// Try to reboot the host, waiting until it is running no tasks.  The reboot entails
// signalling the worker, which will begin an orderly shutdown, so no further tasks
// will be started once this function returns.
func (pl *plugin) reboot() {
	// Try to get a write lock on the mutex; this will wait until all read locks
	// (running tasks) are complete, and will preclude any later read locks.
	pl.runningTasks.Lock()

	// By this time, we should have stopped accepting new tasks, so it is safe
	// to unlock and continue any task completion
	defer pl.runningTasks.Unlock()

	// Reboot and/or die trying. The lock is still held, so no other tasks can
	// start while this process plays out
	pl.environment.Monitor.Warn("Initiating reboot")
	initiateReboot(pl.rebootCommand)
}

func (*plugin) PayloadSchema() schematypes.Object {
	return payloadSchema
}

func (pl *plugin) NewTaskPlugin(options plugins.TaskPluginOptions) (plugins.TaskPlugin, error) {
	var p payloadType
	err := payloadSchema.Map(options.Payload, &p)
	if err == schematypes.ErrTypeMismatch {
		panic("internal error -- type mismatch")
	} else if err != nil {
		return nil, fmt.Errorf("While reading reboot plugin payload: %s", err)
	}

	pl.runningTasks.RLock()
	return &taskPlugin{
		TaskPluginBase: plugins.TaskPluginBase{},
		plugin:         pl,
		rebootWhenDone: p.RebootWhenDone,
	}, nil
}

func (tp *taskPlugin) Dispose() error {
	tp.plugin.runningTasks.RUnlock()

	// Note: in a multi-task environment, this will not reboot immediately, but
	// will wait until all tasks are complete.
	if tp.rebootWhenDone {
		tp.plugin.environment.Monitor.Warn("Task specified reboot after completion; scheduling reboot")
		tp.plugin.reboot()
	}

	return nil
}
