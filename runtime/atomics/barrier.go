package atomics

import (
	"context"
	"sync"
	"time"
)

// A Barrier is an atomic primitive that can be unblocked once, after which it
// stays permanently unblocked. Useful for communicating the permanent state
// changes like shutdown.
type Barrier struct {
	m         sync.Mutex
	b         chan struct{}
	callbacks []func()
}

func (b *Barrier) init() {
	b.m.Lock()
	defer b.m.Unlock()

	if b.b == nil {
		b.b = make(chan struct{})
	}
}

// Fall lowers the barrier permanently unblocking anyone waiting for the barrier
func (b *Barrier) Fall() {
	b.init()

	b.m.Lock()
	defer b.m.Unlock()

	select {
	case <-b.b:
	default:
		for _, cb := range b.callbacks {
			cb()
		}
		b.callbacks = nil
		close(b.b)
	}
}

// IsFallen returns true, if the barrier is lowered.
func (b *Barrier) IsFallen() bool {
	select {
	case <-b.Barrier():
		return true
	default:
		return false
	}
}

// Barrier returns a channel that is closed when the barrier is lowered.
func (b *Barrier) Barrier() <-chan struct{} {
	b.init()
	return b.b
}

// Forward ensures that cb is called when b is lowered.
//
// If already lowered, cb will be called immediately.
// This can be used to lower another barrier when b is lowered as follows:
//   var b1 atomics.Barrier
//   var b2 atomics.Barrier
//   b1.Forward(b2.Fall)
//   b1.Fall() // Lowers both b1 and b2
func (b *Barrier) Forward(cb func()) {
	b.init()

	b.m.Lock()
	defer b.m.Unlock()

	select {
	case <-b.b:
		cb()
	default:
		b.callbacks = append(b.callbacks, cb)
	}
}

// AsContext returns the Barrier as a context.Context that is canceled when the
// barrier is lowered.
func (b *Barrier) AsContext() context.Context {
	return &barrierAsContext{b}
}

type barrierAsContext struct {
	b *Barrier
}

func (c *barrierAsContext) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

func (c *barrierAsContext) Done() <-chan struct{} {
	return c.b.Barrier()
}

func (c *barrierAsContext) Err() error {
	if c.b.IsFallen() {
		return context.Canceled
	}
	return nil
}

func (c *barrierAsContext) Value(interface{}) interface{} {
	return nil
}
