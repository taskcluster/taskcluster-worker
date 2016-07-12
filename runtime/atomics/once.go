package atomics

import "sync"

// Once is similar to sync.Done except that once.Do() returns true, if this
// was the first call to once.Do().
// Additionally a method once.Wait() have been added for anyone waiting for
// this once.Do() to have been called.
//
// Also once.Do(nil) will not panic, but act similar to once.Do(func(){}).
type Once struct {
	m       sync.Mutex
	c       chan struct{}
	started bool
}

// Do will call f() and return true, the first time once.Do() is called.
// All following callls to once.Do() will not call f() and return false.
func (o *Once) Do(f func()) bool {
	// Lock so that we only create one channel
	o.m.Lock()
	// Lazily initialize channel
	if o.c == nil {
		o.c = make(chan struct{})
	}

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

// Wait will block until once.Do() have been called once. After this once.Wait()
// will always return immediately.
func (o *Once) Wait() {
	// Lock so that we only create one channel
	o.m.Lock()
	// Lazily initialize channel
	if o.c == nil {
		o.c = make(chan struct{})
	}
	o.m.Unlock()

	// Wait for channel to be closed
	<-o.c
}
