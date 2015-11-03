package runtime

import "github.com/taskcluster/taskcluster-worker/runtime/gc"

// The EngineContext structure contains generic runtime objects exposed to
// engines. This is largely to simplify implemention of engines, but also to
// ensure that cacheable resources can be managed a single global garbage
// collector.
//
// This context contains runtime objects that are available across all task runs
// and sandboxes. For task run speific properties
type EngineContext struct {
	garbageCollector *gc.GarbageCollector
	//TODO: Add some sort of interface to the system logger, unless the library
	// we choose is global...
	//TODO: Add some interface to submit statistics for influxdb
}

// GarbageCollector returns a gc.GarbageCollector that engines can use to have
// cacheable resources tracked and disposed when the system is low on resources.
func (c *EngineContext) GarbageCollector() *gc.GarbageCollector {
	return c.garbageCollector
}
