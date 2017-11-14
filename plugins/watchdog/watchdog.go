package watchdog

import (
	"errors"
	"sync"
	"time"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
)

// default timeout, setting to 45 minutes is safe, notice that downloading
// images could take 10-15 minutes in some extreme cases.
const defaultTimeout = 45 // minutes

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
	done        atomics.Once
	running     int
}

type taskPlugin struct {
	plugins.TaskPluginBase
	Plugin  *plugin
	started atomics.Once
	stopped atomics.Once
}

func init() {
	plugins.Register("watchdog", provider{})
}

func (provider) ConfigSchema() schematypes.Schema {
	return configSchema
}

func (provider) NewPlugin(options plugins.PluginOptions) (plugins.Plugin, error) {
	var c config
	schematypes.MustValidateAndMap(configSchema, options.Config, &c)

	// Ensure that a default value is set
	if c.Timeout == 0 {
		debug("watchdog configured with default timeout")
		c.Timeout = defaultTimeout * time.Minute
	}

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
	case <-p.done.Done():
	case <-p.timer.C:
		var reported atomics.Once
		go reported.Do(func() {
			p.Monitor.ReportError(errors.New("watchdog: worker timeout exceeded"))
		})

		// Call stopNow within 30s, even if the ReportError blocks and fails...
		// this is just extra hardening...
		select {
		case <-reported.Done():
		case <-time.After(30 * time.Second):
		}
		p.Environment.Worker.StopNow()
	}
}

func (p *plugin) Touch() {
	p.Monitor.Debug("watchdog touched")

	p.m.Lock()
	defer p.m.Unlock()

	p.timer.Stop()
	if p.running == 0 {
		p.timer.Reset(p.Timeout)
	} else {
		p.Monitor.Debugf("watchdog timer paused, currently running: %d", p.running)
	}
}

func (p *plugin) Increment() {
	p.m.Lock()
	p.running++
	p.Monitor.Debugf("watchdog running task incremented to: %d", p.running)
	p.m.Unlock()

	p.Touch()
}

func (p *plugin) Decrement() {
	p.m.Lock()
	p.running--
	p.Monitor.Debugf("watchdog running task decremented to: %d", p.running)
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
	p.done.Do(nil)
	p.timer.Stop()

	return nil
}

func (p *taskPlugin) BuildSandbox(sandboxBuilder engines.SandboxBuilder) error {
	p.Plugin.Touch()
	return nil
}

func (p *taskPlugin) Started(sandbox engines.Sandbox) error {
	// Increment running, and remember this
	// The use of atomics.Once is just to ensure robustness... worker should never
	// call started twice. But we have to remember if started was called in
	// exception and dispose, otherwise, we don't know if we should decrement
	p.started.Do(p.Plugin.Increment)
	return nil
}

func (p *taskPlugin) Stopped(result engines.ResultSet) (bool, error) {
	if p.started.IsDone() {
		p.stopped.Do(p.Plugin.Decrement)
	} else {
		// if started wasn't called then that's contract violation
		panic("TaskPlugin.Stopped() called before TaskPlugin.Started()")
	}
	return true, nil
}

func (p *taskPlugin) Finished(success bool) error {
	p.Plugin.Touch()
	return nil
}

func (p *taskPlugin) Exception(reason runtime.ExceptionReason) error {
	// decrement only if started was called
	if p.started.IsDone() {
		p.stopped.Do(p.Plugin.Decrement)
	}
	p.Plugin.Touch()
	return nil
}

func (p *taskPlugin) Dispose() error {
	// decrement only if started was called
	if p.started.IsDone() {
		p.stopped.Do(p.Plugin.Decrement)
	}
	p.Plugin.Touch()
	return nil
}
