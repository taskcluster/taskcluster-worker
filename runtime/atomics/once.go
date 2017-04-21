package atomics

import "sync"

// Once is similar to sync.Done except that once.Do() returns true, if this
// was the first call to once.Do().
// Additionally, a methods once.Wait(), once.Done(), and once.IsDone() have been
// added for anyone waiting for this once.Do() to have been called.
//
// Also once.Do(nil) will not panic, but act similar to once.Do(func(){}).
type Once struct {
	m       sync.Mutex
	c       chan struct{}
	started bool
}

func (o *Once) initAndLock() {
	// Lock so that we only create one channel
	o.m.Lock()

	// Lazily initialize channel
	if o.c == nil {
		o.c = make(chan struct{})
	}
}

// Do will call f() and return true, the first time once.Do() is called.
// All following callls to once.Do() will not call f() and return false.
func (o *Once) Do(f func()) bool {
	o.initAndLock()

	// If already started, we just return false
	if o.started {
		o.m.Unlock()
		return false
	}

	// Set started true and release lock, so can have nested Do invocations
	o.started = true
	o.m.Unlock()

	// Close channel if anyone is waiting
	defer close(o.c)

	// Do f() and return true, indicating this was the call that did it
	if f != nil {
		f()
	}
	return true
}

// Wait will block until once.Do() have been called, and this call has returned.
// After first once.Do() call once.Wait() will always return immediately.
//
// This is just short-hand for <-once.Done().
func (o *Once) Wait() {
	<-o.Done()
}

// Done returns a channel that is closed when once.Do(fn) have been called and
// fn() has returned.
func (o *Once) Done() <-chan struct{} {
	o.initAndLock()
	o.m.Unlock()

	return o.c
}

// IsDone returns true, if once.Do() have been called, this is non-blocking.
//
// This is just short-hand for non-blocking select on once.Done().
func (o *Once) IsDone() bool {
	select {
	case <-o.Done():
		return true
	default:
		return false
	}
}
