package taskrun

import (
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
)

// Options required to create a TaskRun
type Options struct {
	Environment runtime.Environment
	Engine      engines.Engine
	Plugin      plugins.Plugin
	Monitor     runtime.Monitor
	TaskInfo    runtime.TaskInfo
	Payload     map[string]interface{}
	Queue       client.Queue
}

// mustBeValid panics if Options contains empty values, this allows us to catch
// bugs early, rather than having to debug and trace throughout the entire stack.
func (o *Options) mustBeValid() {
	if o.Environment.GarbageCollector == nil {
		panic("taskrun: Options.Environment.GarbageCollector is nil")
	}
	if o.Environment.Monitor == nil {
		panic("taskrun: Options.Environment.Monitor is nil")
	}
	if o.Environment.TemporaryStorage == nil {
		panic("taskrun: Options.Environment.TemporaryStorage is nil")
	}
	if o.Environment.WebHookServer == nil {
		panic("taskrun: Options.Environment.WebHookServer is nil")
	}
	if o.Engine == nil {
		panic("taskrun: Options.Engine is nil")
	}
	if o.Plugin == nil {
		panic("taskrun: Options.Plugin is nil")
	}
	if o.Monitor == nil {
		panic("taskrun: Options.Monitor is nil")
	}
	if o.TaskInfo == (runtime.TaskInfo{}) {
		panic("taskrun: Options.TaskInfo is empty")
	}
	if o.Payload == nil {
		panic("taskrun: Options.Payload is nil")
	}
	if o.Queue == nil {
		panic("taskrun: Options.Queue is nil")
	}
}
