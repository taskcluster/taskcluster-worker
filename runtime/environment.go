package runtime

import (
	logger "github.com/taskcluster/taskcluster-worker/log"
	"github.com/taskcluster/taskcluster-worker/runtime/gc"
)

// Environment is a collection of objects that makes up a runtime environment.
type Environment struct {
	GarbageCollector *gc.GarbageCollector
	//TODO: Add some sort of interface to the system logger
	//TODO: Add some interface to submit statistics for influxdb/signalfx
	//TODO: Add some interface to attach a http.Handler to public facing server
	TemporaryStorage TemporaryStorage
	Logger           *logger.Logger
}
