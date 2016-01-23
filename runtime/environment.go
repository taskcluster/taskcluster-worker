package runtime

import "github.com/taskcluster/taskcluster-worker/runtime/gc"

type Environment struct {
	GarbageCollector *gc.GarbageCollector
	//TODO: Add some sort of interface to the system logger
	//TODO: Add some interface to submit statistics for influxdb/signalfx
	//TODO: Add some interface to attach a http.Handler to public facing server
	//TODO: Add some interface to create a temporary folder and remove it again...
}
