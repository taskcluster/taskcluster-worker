package caching

import (
	"errors"
	"sync"
)

// Handle holds a reference to a Resource until Release() is called
type Handle struct {
	m        sync.Mutex
	entry    *cacheEntry
	released bool
}

// Resource returns the resource held by the Handle
func (h *Handle) Resource() Resource {
	h.m.Lock()
	defer h.m.Unlock()

	if h.released {
		panic(errors.New("caching.Handle.Release() have been called releasing the resource"))
	}

	return h.entry.resource
}

// Release releases the resource held by the Handle, it safe to call this repeatedly
func (h *Handle) Release() {
	h.m.Lock()
	defer h.m.Unlock()

	if h.released {
		return // Don't release twice!
	}
	h.released = true
	h.entry.release()
}
