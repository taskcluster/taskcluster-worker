package atomics

import "sync"

// Once is similar to sync.Done except that once.Do() returns true, if this
// was the first call to once.Do().
// Additionally a method once.Wait() have been added for anyone waiting for
// this once.Do() to have been called.
//
// Also once.Do(nil) will not panic, but act similar to once.Do(func(){}).
type Once struct {
	wg   sync.WaitGroup
	m    sync.Mutex
	done Bool
	c    chan struct{}
}

// Do will call f() and return true, the first time once.Do() is called.
// All following callls to once.Do() will not call f() and return false.
func (o *Once) Do(f func()) bool {
	// Quickly check if done
	if o.done.Get() {
		return false
	}

	// Lock so that we don't set done twice!
	o.m.Lock()
	defer o.m.Unlock()

	// Check that we're not done
	if o.done.Get() {
		return false
	}

	// Close channel if anyone is waiting
	defer func() {
		if o.c != nil {
			close(o.c)
		}
	}()

	// Set done regardless of panic
	defer o.done.Set(true)

	// Do f() and return true, indicating this was the call that did it
	if f != nil {
		f()
	}
	return true
}

// Wait will block until once.Do() have been called once. After this once.Wait()
// will always return immediately.
func (o *Once) Wait() {
	// Quickly check if done
	if o.done.Get() {
		return
	}

	// Lock so that o.done doesn't change while we create the channel
	o.m.Lock()
	// If done change as we acquired the lock, we're done
	if o.done.Get() {
		o.m.Unlock()
		return
	}
	// Lazily initialize channel
	if o.c == nil {
		o.c = make(chan struct{})
	}
	o.m.Unlock()

	// Wait for channel to be closed
	<-o.c
}
