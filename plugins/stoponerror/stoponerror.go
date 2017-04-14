// Package stoponerror implements a very simple plugin that stops the worker
// gracefully if an non-fatal error is encountered.
package stoponerror

import (
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type pluginProvider struct {
	plugins.PluginProviderBase
}

type plugin struct {
	plugins.PluginBase
	worker  runtime.Stoppable
	monitor runtime.Monitor
}

func init() {
	plugins.Register("stoponerror", pluginProvider{})
}

func (pluginProvider) NewPlugin(options plugins.PluginOptions) (plugins.Plugin, error) {
	return &plugin{
		monitor: options.Monitor,
		worker:  options.Environment.Worker,
	}, nil
}

func (p *plugin) ReportNonFatalError() {
	p.monitor.Info("worker has reported a non-fatal so we are stopping gracefully")
	p.worker.StopGracefully()
}
