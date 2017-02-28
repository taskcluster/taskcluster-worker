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
