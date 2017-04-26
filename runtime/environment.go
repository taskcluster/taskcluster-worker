package runtime

import (
	"github.com/taskcluster/taskcluster-worker/runtime/gc"
	"github.com/taskcluster/taskcluster-worker/runtime/webhookserver"
)

// Environment is a collection of objects that makes up a runtime environment.
//
// This type is intended to be passed by value, and should only contain pointers
// and interfaces for that reason.
type Environment struct {
	GarbageCollector gc.ResourceTracker
	TemporaryStorage
	webhookserver.WebHookServer // Optional, may be nil if not available
	Monitor
	Worker Stoppable
}
