package atomics

import "sync"

// Counter can be changed atomically and conditionally waited on.
type Counter struct {
	m       sync.Mutex
	c       sync.Cond
	value   int
	changed chan struct{}
}

func (c *Counter) init() {
	if c.c.L == nil {
		c.c.L = &c.m
	}
}

// Add value to counter
func (c *Counter) Add(value int) {
	if value == 0 {
		return
	}

	c.m.Lock()
	defer c.m.Unlock()
	c.init()

	c.value += value
	if c.changed != nil {
		close(c.changed)
		c.changed = nil
	}
	c.c.Broadcast()
}

// Value of the counter
func (c *Counter) Value() int {
	c.m.Lock()
	defer c.m.Unlock()
	c.init()

	return c.value
}

// WaitFor predicate over the value to be true
func (c *Counter) WaitFor(predicate func(val int) bool) {
	c.m.Lock()
	defer c.m.Unlock()
	c.init()

	for !predicate(c.value) {
		c.c.Wait()
	}
}

// WaitForLessThan blocks until the counter is less than val
func (c *Counter) WaitForLessThan(val int) {
	c.WaitFor(func(v int) bool {
		return v < val
	})
}

// WaitForZero blocks until counter has reach zero
func (c *Counter) WaitForZero() {
	c.WaitFor(func(val int) bool {
		return val == 0
	})
}

// Changed returns a channel that is closed when the Counter is changed
// without leaking if the counter is never changed.
func (c *Counter) Changed() <-chan struct{} {
	c.m.Lock()
	defer c.m.Unlock()
	c.init()

	if c.changed == nil {
		c.changed = make(chan struct{})
	}
	return c.changed
}
