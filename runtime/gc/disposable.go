package gc

import (
	"errors"
	"time"
)

var (
	// ErrDisposableSizeNotSupported is used by Disposable to indicate that a resource
	// doesn't support a size function like MemorySize() or DiskSize()
	ErrDisposableSizeNotSupported = errors.New("Disposable instance doesn't support")
	// ErrDisposableInUse is used by Disposable to indicate that a resource is currently in
	// use and cannot be disposed
	ErrDisposableInUse = errors.New("Disposable is currently in use")
)

// The Disposable Interface to be implemented by resources that can be managed by the
// GarbageCollector. To have the resource garbage collected you must call
// GarbageCollector.Register(&resource). The garbage collector will call
// resource.Dispose() when it wants to remove your resource, you must check
// that the resource isn't in use and return ErrDisposableInUse if it is.
//
// Warning, return any error other than ErrDisposableSizeNotSupported or
// ErrDisposableInUse and it will result in a internal error aborting all tasks,
// reporting to operators and crashing the worker. This is because we can't
// have workers leak resources, so errors freeing resources has to be critical
// feel free to do retries and other things to avoid returning an error that
// we can't handle.
//
// Note, all methods on this must be thread-safe, an implementation of most
// of these methods can be obtained by extending DisposableResource.
type Disposable interface {
	// Get memory size used by resource, return ErrDisposableSizeNotSupported
	// if you don't know how much memory it uses, but knows that it uses some
	// non-trivial amount. If it just uses a small fixed amount, you can return
	// 1 for simplicity.
	MemorySize() (uint64, error)
	// Get disk space used by resource, return ErrDisposableSizeNotSupported
	// if you don't know how much disk space it uses, but knows that it uses some
	// non-trivial amount. If it just uses a small fixed amount, you can return
	// 1 for simplicity.
	DiskSize() (uint64, error)
	// Clean up after this resource, return ErrDisposableInUse if the resource is
	// in use. Note that implementors should use this method to also remove any
	// references to this resource from the resource manager, which typically has
	// an internal list of cached resources.
	//
	// Warning, return an error other than ErrDisposableInUse and the worker will
	// panic, abort tasks, alert operation and crash.
	Dispose() error
	// Last time the cache was used
	LastUsed() time.Time
}
