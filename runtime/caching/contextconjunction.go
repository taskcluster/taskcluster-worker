package caching

import (
	"context"
	"sync"
	"time"

	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

// a contextConjunction is conjunction of contexts, so which additional contexts
// may be added later... When all the contexts have been canceled or dispose()
// have been called, the contextConjunction will also be canceled.
//
// Notice: dispose() must be called to avoid leaking resources.
type contextConjunction struct {
	m        sync.Mutex
	resolved atomics.Once
	contexts []Context
	err      error
}

// newContextConjunction creates a new contextConjunction starting with ctx
//
// Notice: dispose() must be called to avoid leaking resources.
func newContextConjunction(ctx Context) *contextConjunction {
	c := &contextConjunction{
		contexts: []Context{ctx},
	}

	go c.awaitContexts()

	return c
}

// dispose() cancels the context and cleanups resources, this must be called.
func (c *contextConjunction) dispose() {
	c.m.Lock()
	defer c.m.Unlock()

	c.resolved.Do(func() {
		c.err = context.Canceled
	})
}

func (c *contextConjunction) awaitContexts() {
	c.m.Lock()
	defer c.m.Unlock()

	var err error
	for i := 0; i < len(c.contexts); i++ {
		ctx := c.contexts[i]
		c.m.Unlock()
		select {
		case <-ctx.Done():
			err = ctx.Err()
		case <-c.resolved.Done():
		}
		c.m.Lock()
	}

	c.resolved.Do(func() {
		c.err = err
	})
}

// AddContext to the conjunction, returns false if the contextConjunction was
// canceled before ctx could be added.
func (c *contextConjunction) AddContext(ctx Context) bool {
	c.m.Lock()
	defer c.m.Unlock()

	if c.resolved.IsDone() {
		return false
	}

	c.contexts = append(c.contexts, ctx)
	return true
}

func (c *contextConjunction) Progress(description string, percent float64) {
	c.m.Lock()
	defer c.m.Unlock()

	util.Spawn(len(c.contexts), func(i int) {
		c.contexts[i].Progress(description, percent)
	})
}

func (c *contextConjunction) Deadline() (deadline time.Time, ok bool) {
	return time.Time{}, false
}

func (c *contextConjunction) Done() <-chan struct{} {
	return c.resolved.Done()
}

func (c *contextConjunction) Err() error {
	c.m.Lock()
	defer c.m.Unlock()

	return c.err
}

func (c *contextConjunction) Value(key interface{}) interface{} {
	return nil
}
