package maxruntime

import (
	"time"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type pluginProvider struct {
	plugins.PluginProviderBase
}

type plugin struct {
	plugins.PluginBase
}

type taskPlugin struct {
	plugins.TaskPluginBase
	maxRunTime int
	done       chan bool
	monitor    runtime.Monitor
	context    *runtime.TaskContext
}

type payloadType struct {
	MaxRunTime int `json:"maxRunTime"`
}

var payloadSchema = schematypes.Object{
	Properties: schematypes.Properties{
		"maxRunTime": schematypes.Integer{
			MetaData: schematypes.MetaData{
				Title:       "Task execution timeout (s)",
				Description: "Kill the task if it exceedes the timeout value.",
			},
			Minimum: 0,
			Maximum: 24 * 60 * 60,
		},
	},
}

func (pluginProvider) NewPlugin(plugins.PluginOptions) (plugins.Plugin, error) {
	return plugin{}, nil
}

func (plugin) PayloadSchema() schematypes.Object {
	return payloadSchema
}

func (plugin) NewTaskPlugin(options plugins.TaskPluginOptions) (plugins.TaskPlugin, error) {
	var p payloadType
	if err := schematypes.MustMap(payloadSchema, options.Payload, &p); err != nil {
		return nil, engines.ErrContractViolation
	}

	return &taskPlugin{
		TaskPluginBase: plugins.TaskPluginBase{},
		maxRunTime:     p.MaxRunTime,
		done:           make(chan bool),
		monitor:        options.Monitor,
	}, nil
}

func (tp *taskPlugin) Prepare(context *runtime.TaskContext) error {
	tp.context = context
	return nil
}

func (tp *taskPlugin) Started(sandbox engines.Sandbox) error {
	go func() {
		select {
		case <-time.After(time.Duration(tp.maxRunTime) * time.Second):
			tp.monitor.Info("Killing task due to maxRunTime exceeded")
			tp.context.LogError("Task was killed because maximum run time was exceeded")
			sandbox.Abort()
		case <-tp.done:
		}
	}()
	return nil
}

func (tp *taskPlugin) Stopped(result engines.ResultSet) (bool, error) {
	close(tp.done)
	return true, nil
}

func init() {
	plugins.Register("maxruntime", pluginProvider{})
}
