package caching

import "github.com/taskcluster/taskcluster-worker/runtime/gc"

// ErrDisposableSizeNotSupported should be returned from DiskSize() and
// MemorySize() if said feature is not supported
var ErrDisposableSizeNotSupported = gc.ErrDisposableSizeNotSupported

// A Resource that can be cached must also be disposable
type Resource interface {
	MemorySize() (uint64, error)
	DiskSize() (uint64, error)
	Dispose() error
}
