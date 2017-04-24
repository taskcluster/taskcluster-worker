package maxruntime

import (
	"time"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/engines"
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
	config
}

type taskPlugin struct {
	plugins.TaskPluginBase
	maxRunTime time.Duration
	monitor    runtime.Monitor
	context    *runtime.TaskContext
	stopped    atomics.Once
	killed     atomics.Bool
}

func init() {
	plugins.Register("maxruntime", provider{})
}

type payload struct {
	MaxRunTime time.Duration `json:"maxRunTime"`
}

func (provider) ConfigSchema() schematypes.Schema {
	return configSchema
}

func (provider) NewPlugin(options plugins.PluginOptions) (plugins.Plugin, error) {
	var c config
	schematypes.MustValidateAndMap(configSchema, options.Config, &c)

	return &plugin{
		config: c,
	}, nil
}

func (p *plugin) PayloadSchema() schematypes.Object {
	s := schematypes.Object{}
	if p.PerTaskLimit == limitAllow || p.PerTaskLimit == limitRequire {
		s.Properties = schematypes.Properties{
			"maxRunTime": schematypes.Duration{
				Title: "Maximum Task Run-Time",
				Description: util.Markdown(`
					The maximum task run-time before the task is **killed** and resolved
					as _failed_. Specified as an integer in seconds, or as string on
					the form: '1 day 2 hours 3 minutes'.

					This is measured as the execution time and does not include time
					the worker spends downloading images or upload artifacts.

					For this worker-type the 'maxRunTime' may not exceed:
					'` + p.MaxRunTime.String() + `'.
				`),
			},
		}
		if p.PerTaskLimit == limitRequire {
			s.Required = []string{"maxRunTime"}
		}
	}
	return s
}

func (p *plugin) NewTaskPlugin(options plugins.TaskPluginOptions) (plugins.TaskPlugin, error) {
	var P struct {
		MaxRunTime time.Duration `json:"maxRunTime"`
	}
	schematypes.MustValidateAndMap(p.PayloadSchema(), options.Payload, &P)

	// Default to globally configured limit
	maxRunTime := P.MaxRunTime
	if maxRunTime == 0 {
		maxRunTime = p.MaxRunTime
	}

	// Return malformed payload if maxRunTime is more than global limit
	if maxRunTime > p.MaxRunTime {
		return nil, runtime.NewMalformedPayloadError(
			"task.payload.maxRunTime may not exceeed", p.MaxRunTime.String(),
			"as is configured the maximum runtime for this workerType",
		)
	}

	return &taskPlugin{
		context:    options.TaskContext,
		monitor:    options.Monitor,
		maxRunTime: maxRunTime,
	}, nil
}

func (p *taskPlugin) Started(sandbox engines.Sandbox) error {
	go func() {
		select {
		case <-time.After(p.maxRunTime):
			// when maxRunTime has elapsed we kill the task
			p.killed.Set(true)
			p.monitor.Info("Killing task due to maxRunTime exceeded")
			p.context.LogError("Task killed because maxRunTime was exceeded")
			sandbox.Kill()
		case <-p.context.Done():
			// when task context is canceled, then we need not kill anything
		case <-p.stopped.Done():
			// when task has stopped, we need not kill anything
		}
	}()
	return nil
}

func (p *taskPlugin) Stopped(engines.ResultSet) (bool, error) {
	p.stopped.Do(nil)
	// If we've killed the task, then we want to force a negative resolution
	return !p.killed.Get(), nil
}

func (p *taskPlugin) Exception(runtime.ExceptionReason) error {
	p.stopped.Do(nil) // shouldn't be necessary, but it's good for robustness
	return nil
}

func (p *taskPlugin) Dispose() error {
	p.stopped.Do(nil) // shouldn't be necessary, but just for good measure
	return nil
}
