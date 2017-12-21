package caching

import (
	"errors"
	"sync"
	"time"

	"github.com/taskcluster/taskcluster-worker/runtime/gc"
)

// A Constructor is function that given options creates a resource.
//
// Notice, options must be JSON serializable, as they will be hashed to
// determine resource equivalence.
type Constructor func(ctx Context, options interface{}) (Resource, error)

// A Cache is an object that wraps a constructor and manages the life-cycle of
// objects. At creation a cache is given a Constructor, a ResourceTracker and
// the option of being an exclusive or shared-cache, meaning if resources may be
// re-used before they are released.
type Cache struct {
	m           sync.Mutex
	shared      bool
	entries     []*cacheEntry
	constructor Constructor
	tracker     gc.ResourceTracker
}

// New returns a Cache wrapping constructor such that resources
// returned from Require are shared between all calls to Require with the same
// options, if shared is set to true. Otherwise, resources are exclusive.
func New(constructor Constructor, shared bool, tracker gc.ResourceTracker) *Cache {
	return &Cache{
		constructor: constructor,
		tracker:     tracker,
		shared:      shared,
	}
}

func (c *Cache) remove(e *cacheEntry) {
	// Lock cache entries
	c.m.Lock()
	defer c.m.Unlock()

	// Remove from cache
	entries := c.entries[:0]
	for _, entry := range c.entries {
		if entry != e {
			entries = append(entries, entry)
		}
	}
	c.entries = entries
}

// Require returns a resource for the given options
//
// If the Cache is exclusive the resource will not be re-used before it is
// returned with a call to Release(resource). If the cache is shared the cache
// will only every create one instance of the resource, unless purged or freed
// by the ResourceTracker.
func (c *Cache) Require(ctx Context, options interface{}) (*Handle, error) {
	optionsHash := hashJSON(options)

	// Lock the entries list
	c.m.Lock()

	// find cache entry, if present
	var entry *cacheEntry
	for _, e := range c.entries {
		e.m.Lock()
		// Ignore if: scheduled to be purged, disposed or options mismatch
		if e.purge || e.disposed || e.optionsHash != optionsHash {
			e.m.Unlock()
			continue
		}
		// Skip if this isn't a shared cache and entry is in-use
		if !c.shared && e.refCount > 0 {
			e.m.Unlock()
			continue
		}
		// Skip if we are too late to join the context (ie. it was canceled), and
		// the resource haven't been created. There is a race here that could
		// cause us to ignore a successfully created resource, but it's unlikely,
		// mostly this is ignoring resource creations that haven't finished, but
		// have been canceled (since we can't uncancel)
		if !e.ctx.AddContext(ctx) && !e.created.IsDone() {
			e.m.Unlock()
			continue
		}

		// Take the entry
		debug("cache entry '%s' found in cache, refCount: %d", optionsHash, e.refCount+1)
		entry = e
		e.refCount++
		e.m.Unlock()
		break
	}

	// Create new resource
	if entry == nil {
		debug("cache entry '%s' is being created", optionsHash)
		entry = &cacheEntry{
			optionsHash: optionsHash,
			refCount:    1,
			lastUsed:    time.Now(),
			ctx:         newContextConjunction(ctx),
			cache:       c,
		}

		// Insert in cache so we can find it again, and re-use the resource
		c.entries = append(c.entries, entry)

		go entry.created.Do(func() {
			defer entry.ctx.dispose() // ensure resources are cleanup when constructor is done

			entry.resource, entry.err = c.constructor(entry.ctx, options)
			if entry.err != nil {
				// Set the entry to be purged, so others will ignore it
				entry.m.Lock()
				entry.purge = true
				entry.disposed = true
				entry.m.Unlock()
				// Remove it from entries, so we don't cache error results
				c.remove(entry)
			} else {
				debug("cache entry '%s' ready with resource type: %T", entry.optionsHash, entry.resource)
				// Insert in garbage collector
				c.tracker.Register(entry)
			}
		})
	}

	// Unlock the entries list, while we wait for the entry to be created
	c.m.Unlock()

	// Wait for the entry to be created
	select {
	case <-entry.created.Done():
		if entry.err != nil {
			entry.release()
			return nil, entry.err
		}
		return &Handle{entry: entry}, nil
	case <-ctx.Done():
		entry.release()
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		panic(errors.New("expected context.Err() != nil when context.Done() is closed"))
	}
}

// Purge cached resources, notice that these resources may be in use.
// The filter function should return true if the resource should be purged,
// then it'll purged when it is no-longer in use.
func (c *Cache) Purge(filter func(r Resource) bool) error {
	c.m.Lock()
	defer c.m.Unlock()

	// find entries to delete
	for _, entry := range c.entries {
		entry.m.Lock()
		ignore := !entry.created.IsDone() || entry.purge || entry.disposed
		entry.m.Unlock()

		if !ignore && filter(entry.resource) {
			// Set the bit for it to be disposed when given back to Release
			entry.m.Lock()
			entry.purge = true
			entry.m.Unlock()

			// Remove entry from ResourceTracker
			c.tracker.Unregister(entry)

			if err := entry.Dispose(); err != nil && err != gc.ErrDisposableInUse {
				return err
			}
		}
	}

	return nil
}

// PurgeAll will purge all resources returning the first error, then proceeding
// and purging everything else, before returning the first error, if any.
func (c *Cache) PurgeAll() error {
	c.m.Lock()
	defer c.m.Unlock()

	// find entries to delete
	var err error
	for _, entry := range c.entries {
		entry.m.Lock()
		ignore := !entry.created.IsDone() || entry.purge || entry.disposed
		entry.m.Unlock()

		if !ignore {
			// Set the bit for it to be disposed when given back to Release
			entry.m.Lock()
			entry.purge = true
			entry.m.Unlock()

			// Remove entry from ResourceTracker
			c.tracker.Unregister(entry)

			if derr := entry.Dispose(); derr != nil && derr != gc.ErrDisposableInUse && err == nil {
				err = derr // return only the first error
			}
		}
	}

	return err
}
