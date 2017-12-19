package caching

import (
	"fmt"
	"sync"
	"time"

	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
	"github.com/taskcluster/taskcluster-worker/runtime/gc"
)

type cacheEntry struct {
	m           sync.Mutex
	optionsHash string
	created     atomics.Once
	refCount    uint32
	lastUsed    time.Time
	ctx         *contextConjunction
	resource    Resource
	err         error
	cache       *Cache
	purge       bool // true, if removed from GC, should not be used and disposed when refCount == 0
	disposed    bool // true, if disposed and just waiting to be removed from cache.entries
}

func (e *cacheEntry) MemorySize() (uint64, error) {
	return e.resource.MemorySize()
}

func (e *cacheEntry) DiskSize() (uint64, error) {
	return e.resource.DiskSize()
}

func (e *cacheEntry) LastUsed() time.Time {
	return e.lastUsed
}

func (e *cacheEntry) Dispose() error {
	e.m.Lock()

	if e.refCount > 0 {
		e.m.Unlock()
		return gc.ErrDisposableInUse
	}

	disposed := e.disposed
	e.purge = true
	e.disposed = true
	e.m.Unlock() // unlock before attempting to remove from cache to avoid deadlock

	// Schedule e to be removed from the cache
	go e.cache.remove(e)

	// Avoid disposing twice
	if disposed {
		return nil
	}

	// Dispose the resource
	return e.resource.Dispose()
}

func (e *cacheEntry) release() {
	e.m.Lock()
	defer e.m.Unlock()

	debug("cache entry '%s' released to cache, refCount: %d", e.optionsHash, e.refCount-1)

	// Make sure we don't go negative
	if e.refCount == 0 {
		panic(fmt.Errorf("cacheEntry had refCount == 0 when decremented for resource type: %T", e.resource))
	}

	// Decrement reference count
	e.refCount--
	e.lastUsed = time.Now()

	// If refCount is zero and this entry is scheduled to be purge, we dispose of it
	if e.refCount == 0 && e.purge {
		disposed := e.disposed
		e.purge = true
		e.disposed = true

		// Schedule e to be removed from the cache
		go e.cache.remove(e)

		// Avoid disposing twice
		if disposed {
			return
		}

		// TODO: Report errors (ignore them for now)
		go e.resource.Dispose()
	}
}
