// Package reboot provides a taskcluster-worker plugin that stops the worker
// after certain number of tasks or given amount of time. This is useful if the
// start-up scripts that launches the worker reboots when it exits.
//
// In some cases, especially running without a very tight
// sandbox, this is desirable after specific tests.
//
// In other cases, reboots are useful after a configured uptime, to
// cycle the host's configuration.
package reboot

import (
	"os/exec"
	"sync"
	"time"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

type provider struct {
	plugins.PluginProviderBase
}

type plugin struct {
	plugins.PluginBase
	Worker     runtime.Stoppable
	Monitor    runtime.Monitor
	Config     config
	mTaskCount sync.Mutex   // guards taskCount
	taskCount  int64        // track number of tasks
	rebooted   atomics.Once // track if this plugin initiated shutdown
}

type taskPlugin struct {
	plugins.TaskPluginBase
	Plugin       *plugin
	Monitor      runtime.Monitor
	RebootAction string
}

func init() {
	plugins.Register("reboot", provider{})
}

func (provider) ConfigSchema() schematypes.Schema {
	return configSchema
}

func (provider) NewPlugin(options plugins.PluginOptions) (plugins.Plugin, error) {
	var C config
	schematypes.MustValidateAndMap(configSchema, options.Config, &C)

	p := &plugin{
		Worker:  options.Environment.Worker,
		Monitor: options.Monitor,
		Config:  C,
	}

	// Start timer to stop worker gracefully, if MaxLifeCycle is given
	if p.Config.MaxLifeCycle != 0 {
		time.AfterFunc(p.Config.MaxLifeCycle, func() {
			// Avoid initiating reboot if we already have
			p.rebooted.Do(func() {
				p.Monitor.Infof("MaxLifeCycle: %s exceeded stopping worker gracefully", p.Config.MaxLifeCycle.String())
				p.Worker.StopGracefully()
			})
		})
	}

	return p, nil
}

const (
	rebootAlways      = "always"
	rebootOnFailure   = "on-failure"
	rebootOnException = "on-exception"
)

func (p *plugin) PayloadSchema() schematypes.Object {
	s := schematypes.Object{}
	if p.Config.AllowTaskReboots {
		s.Properties = schematypes.Properties{
			"reboot": schematypes.StringEnum{
				Title: "Reboot After Task",
				Description: util.Markdown(`
					Reboot the worker after this task is resolved.

					This option is useful if the task is known to leave the worker in
					a dirty state. To spare resources it is possible to condition the
					reboot on the task resolution, allowed values are:

						* 'always', reboots the worker after the task is resolved,
						* 'on-failure', reboots the worker if the task is resolved
						_failed_ or _exception_.
						* 'on-exception', reboots the worker if the tsak is resolved
						_exception_.

					Regardless of which option is given the worker will always reboot
					gracefully, reporting task resolution and uploading artifacts before
					rebooting.
				`),
				Options: []string{rebootAlways, rebootOnFailure, rebootOnException},
			},
		}
	}
	return s
}

func (p *plugin) Dispose() error {
	if p.rebooted.IsDone() && len(p.Config.RebootCommand) > 0 {
		p.Monitor.Infof("worker shutdown initiated by 'reboot' plugin, running rebootCommand: %v", p.Config.RebootCommand)
		// Run the reboot command
		log, err := exec.Command(p.Config.RebootCommand[0], p.Config.RebootCommand[1:]...).CombinedOutput()
		if err != nil {
			p.Monitor.Error("rebootCommand failed, error: ", err, ", log: ", string(log))
		} else {
			p.Monitor.Info("rebootCommand executed, log: ", string(log))
		}
	}
	return nil
}

func (p *plugin) NewTaskPlugin(options plugins.TaskPluginOptions) (plugins.TaskPlugin, error) {
	var P struct {
		Reboot string `json:"reboot"`
	}
	schematypes.MustValidateAndMap(p.PayloadSchema(), options.Payload, &P)

	// Increase taskCount, and StopGracefully if we've exceeded TaskLimit
	p.mTaskCount.Lock()
	defer p.mTaskCount.Unlock()
	p.taskCount++
	if p.Config.TaskLimit != 0 && p.taskCount >= p.Config.TaskLimit {
		// Avoid initiating reboot if we already have
		p.rebooted.Do(func() {
			p.Monitor.Infof("Stopping worker after %d tasks as configured", p.Config.TaskLimit)
			p.Worker.StopGracefully()
		})
	}

	return &taskPlugin{
		Plugin:       p,
		Monitor:      options.Monitor,
		RebootAction: P.Reboot,
	}, nil
}

func (p *taskPlugin) Finished(success bool) error {
	if p.RebootAction == rebootAlways || (!success && p.RebootAction == rebootOnFailure) {
		// Avoid initiating reboot if we already have
		p.Plugin.rebooted.Do(func() {
			p.Monitor.Infof("Stopping worker after task as instructed by task.payload.reboot = '%s'", p.RebootAction)
			p.Plugin.Worker.StopGracefully()
		})
	}
	return nil
}

func (p *taskPlugin) Exception(runtime.ExceptionReason) error {
	if p.RebootAction == rebootAlways || p.RebootAction == rebootOnFailure || p.RebootAction == rebootOnException {
		// Avoid initiating reboot if we already have
		p.Plugin.rebooted.Do(func() {
			p.Monitor.Infof("Stopping worker after task as instructed by task.payload.reboot = '%s'", p.RebootAction)
			p.Plugin.Worker.StopGracefully()
		})
	}
	return nil
}
