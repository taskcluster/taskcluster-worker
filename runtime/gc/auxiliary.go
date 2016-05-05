//BUG(jonasfj) Not sure this base struct is useful at all...

package gc

import (
	"sync"
	"time"
)

// The DisposableResource implements as thread-safe reference counted resource
// for the Disposable interface. Such that Acquire/Release updates the LastUsed
// time stamp.
//
// Implementors of the Disposable interface using this base struct should
// implement Dispose such that it returns ErrDisposableSizeNotSupported if
// currently in use. This can be easily actieved using CanDispose.
//
// Implementor should also implement EstimateMemorySize() and EstimateDiskSize(), but these are
// unrelated to this trait.
type DisposableResource struct {
	refCount uint32
	lastUsed time.Time
	m        sync.Mutex
}

// Acquire the resource incrementing the reference count by one
func (r *DisposableResource) Acquire() {
	r.m.Lock()
	defer r.m.Unlock()
	r.refCount++
}

// Release the resource decrementing the reference count by one and updating
// the lastUsed time stamp to now.
func (r *DisposableResource) Release() {
	r.m.Lock()
	defer r.m.Unlock()
	r.refCount--
	r.lastUsed = time.Now()
}

// CanDispose returns ErrDisposableInUse if the resource is currently
// being used. This is intended to be used by implementors of Dispose.
func (r *DisposableResource) CanDispose() error {
	r.m.Lock()
	defer r.m.Unlock()
	if r.refCount > 0 {
		return ErrDisposableInUse
	}
	return nil
}

// LastUsed returns the last time this resource was used, as an implementation
// for the Disposable interface
func (r *DisposableResource) LastUsed() time.Time {
	r.m.Lock()
	defer r.m.Unlock()
	return r.lastUsed
}

// MemorySize is the stub implementation of Disposable.MemorySize returning
// ErrDisposableSizeNotSupported, implementors really ought to overwrite this.
func (r *DisposableResource) MemorySize() (uint64, error) {
	return 0, ErrDisposableSizeNotSupported
}

// DiskSize is the stub implementation of Disposable.DiskSize returning
// ErrDisposableSizeNotSupported, implementors really ought to overwrite this.
func (r *DisposableResource) DiskSize() (uint64, error) {
	return 0, ErrDisposableSizeNotSupported
}
