package runtime

import (
	"github.com/taskcluster/taskcluster-worker/runtime/gc"
	"github.com/taskcluster/taskcluster-worker/runtime/webhookserver"
)

// Environment is a collection of objects that makes up a runtime environment.
type Environment struct {
	GarbageCollector gc.ResourceTracker
	TemporaryStorage
	webhookserver.WebHookServer
	Monitor
}

// Stoppable is an worker with a life-cycle that can be can be stopped.
type Stoppable interface {
	// StopNow causes the worker to stop processing tasks, resolving all active
	// tasks exception w. worker-shutdown.
	StopNow()
	// StopGracefully causes the worker to stop claiming tasks and stop gracefully
	// when all active tasks have been resolved.
	StopGracefully()
}
