package gc

import "sync"

func indexOfResource(resources []Disposable, resource Disposable) int {
	for i, r := range resources {
		if r == resource {
			return i
		}
	}
	return -1
}

// GarbageCollector can be used register Disposable resources which will then
// be diposed when not in use and the system is low on available disk space
// or memory.
type GarbageCollector struct {
	resources []Disposable
	m         sync.Mutex
}

// Register takes a Disposable resource for the GarbageCollector to manage.
//
// GarbageCollector will attempt to to call resource.Dispose() at any time,
// you should return ErrDisposableInUse if the resource is in use.
func (gc *GarbageCollector) Register(resource Disposable) {
	gc.m.Lock()
	defer gc.m.Unlock()
	if indexOfResource(gc.resources, resource) != -1 {
		gc.resources = append(gc.resources, resource)
	}
}

// Unregister will inform the GarbageCollector to stop tracking the given
// resource. Returns true if the resource was tracked, notice that the resource
// maybe have been disposed of while waiting for the GC lock. Say if the GC was
// running when you made this call.
//
// Note, you don't have to use this method. When you resouce is in a state
// where you don't want it to be disposed just ensure that Dispose() returns
// ErrDisposableInUse.
func (gc *GarbageCollector) Unregister(resource Disposable) bool {
	gc.m.Lock()
	defer gc.m.Unlock()
	i := indexOfResource(gc.resources, resource)
	if i != -1 {
		gc.resources = append(gc.resources[:i], gc.resources[i+1:]...)
		return true
	}
	return false
}

// Collect runs garbage collection and reclaims resources, at this stage it just
// disposes as many resources as possible.
func (gc *GarbageCollector) Collect() error {
	gc.m.Lock()
	defer gc.m.Unlock()
	var resources []Disposable
	for i, resource := range gc.resources {
		err := resource.Dispose()
		if err != nil {
			if err != ErrDisposableInUse {
				gc.resources = append(resources, gc.resources[i-1:]...)
				return err
			}
			resources = append(resources, resource)
		}
	}
	gc.resources = resources
	return nil
}
