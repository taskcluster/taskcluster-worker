package watchdog

import (
	"errors"
	"strconv"
	"sync"
	"time"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
)

// default timeout, setting to 30 minutes is safe, notice that downloading
// images could take 10-15 minutes in some extreme cases.
const defaultTimeout = 30 // minutes

type provider struct {
	plugins.PluginProviderBase
}

type plugin struct {
	plugins.PluginBase
	Timeout     time.Duration
	Environment runtime.Environment
	Monitor     runtime.Monitor
	m           sync.Mutex
	timer       *time.Timer
	done        atomics.Barrier
	running     int
}

type taskPlugin struct {
	plugins.TaskPluginBase
	Plugin  *plugin
	stopped atomics.Barrier
}

func init() {
	plugins.Register("watchdog", provider{})
}

var configSchema = schematypes.Object{
	MetaData: schematypes.MetaData{
		Title: "Watchdog Plugin",
		Description: `
      The watchdog plugin resets a timer whenever the worker is reported as
      idle or processes a step in a task. This ensure that the task-processing
      loop remains alive. If the timeout is exceeded, the watchdog will report
      to sentry and shutdown the worker immediately.

      This plugin is mainly useful to avoid stale workers cut in some livelock.
      Note: that this plugin won't cause a timeout between Started() and
      Stopped(), as this would limit task run time, for this purpose use the
      'maxruntime' plugin.
    `,
	},
	Properties: schematypes.Properties{
		"timeout": schematypes.Duration{
			MetaData: schematypes.MetaData{
				Title: "Watchdog Timeout",
				Description: `
          Timeout after which to kill the worker, timeout is reset whenever a
          task progresses, worker is reported idle or task is between Started()
          and Stopped().

          Defaults to ` + strconv.Itoa(defaultTimeout) + ` minutes, if not
          specified (or zero).

          This property is specified in seconds as integer or as string on the
          form '1 day 2 hours 3 minutes'.
        `,
			},
			AllowNegative: false,
		},
	},
}

func (provider) ConfigSchema() schematypes.Schema {
	return configSchema
}

func (provider) NewPlugin(options plugins.PluginOptions) (plugins.Plugin, error) {
	var c struct {
		Timeout time.Duration `json:"timeout"`
	}
	schematypes.MustValidateAndMap(configSchema, options.Config, &c)

	p := &plugin{
		Timeout:     c.Timeout,
		Environment: *options.Environment,
		Monitor:     options.Monitor,
		timer:       time.NewTimer(c.Timeout),
	}
	go p.waitForTimeout()

	return p, nil
}

func (p *plugin) waitForTimeout() {
	select {
	case <-p.done.Barrier():
	case <-p.timer.C:
		reported := atomics.Barrier{}
		go func() {
			defer reported.Fall()
			p.Monitor.ReportError(errors.New("watchdog: worker timeout exceeded"))
		}()
		// Call stopNow within 30s, even if the ReportError blocks and fails...
		// this is just extra hardening...
		select {
		case <-reported.Barrier():
		case <-time.After(30 * time.Second):
		}
		p.Environment.Worker.StopNow()
	}
}

func (p *plugin) Touch() {
	p.Monitor.Info("watchdog touched")

	p.m.Lock()
	defer p.m.Unlock()

	p.timer.Stop()
	if p.running == 0 {
		p.timer.Reset(p.Timeout)
	} else {
		p.Monitor.Debugf("watchdog timer paused, currently running: %d", p.running)
	}
}

func (p *plugin) AddRunning(count int) {
	p.Monitor.Debugf("watchdog running task count change: %d", count)

	p.m.Lock()
	p.running += count
	p.m.Unlock()

	p.Touch()
}

func (p *plugin) ReportIdle(time.Duration) {
	p.Touch()
}

func (p *plugin) NewTaskPlugin(plugins.TaskPluginOptions) (plugins.TaskPlugin, error) {
	p.Touch()
	return &taskPlugin{
		Plugin: p,
	}, nil
}

func (p *plugin) Dispose() error {
	p.m.Lock()
	defer p.m.Unlock()

	// Stop waitForTimeout
	p.done.Fall()
	p.timer.Stop()

	return nil
}

func (p *taskPlugin) BuildSandbox(sandboxBuilder engines.SandboxBuilder) error {
	p.Plugin.Touch()
	return nil
}

func (p *taskPlugin) Started(sandbox engines.Sandbox) error {
	// Increment running and ensure that we decrement no-matter how we leave...
	p.Plugin.AddRunning(1)
	p.stopped.Forward(func() {
		p.Plugin.AddRunning(-1)
	})
	return nil
}

func (p *taskPlugin) Stopped(result engines.ResultSet) (bool, error) {
	p.stopped.Fall()
	return true, nil
}

func (p *taskPlugin) Finished(success bool) error {
	p.Plugin.Touch()
	return nil
}

func (p *taskPlugin) Exception(reason runtime.ExceptionReason) error {
	p.stopped.Fall()
	p.Plugin.Touch()
	return nil
}

func (p *taskPlugin) Dispose() error {
	p.stopped.Fall()
	p.Plugin.Touch()
	return nil
}
