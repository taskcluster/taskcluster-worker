package atomics

import (
	"errors"
	"sync"
)

// ErrWaitGroupDraining is returned from WaitGroup.Add(), if WaitGroup is in
// draining state.
var ErrWaitGroupDraining = errors.New(
	"WaitGroup is draining, internal counter can no-longer be incremented",
)

// WaitGroup is similar to sync.WaitGroup, except it can enter a draining state
// at which point additional calls to Add will fail and returns
// ErrWaitGroupDraining
type WaitGroup struct {
	m        sync.Mutex
	c        sync.Cond
	count    int
	draining bool
}

func (wg *WaitGroup) testAndBroadcast() {
	// Lock must be held when this is called

	// Test if there is anything to do
	if wg.count <= 0 {
		if wg.count < 0 {
			panic("Internal counter in atomics.WaitGroup may not go negative")
		}

		// Skip Broadcast if nobody is waiting
		if wg.c.L != nil {
			wg.c.Broadcast()
		}
	}
}

// Add will increment internal counter by delta, if not in draining state.
// If draining, Add(delta) return ErrWaitGroupDraining if delta is positive.
//
// If the internal counter goes negative Add will panic.
func (wg *WaitGroup) Add(delta int) error {
	wg.m.Lock()
	defer wg.m.Unlock()

	// Abort, if currently draining
	if delta > 0 && wg.draining {
		return ErrWaitGroupDraining
	}

	wg.count += delta
	wg.testAndBroadcast()

	return nil
}

// Done decrements internal counter and unblocks Wait() when it counter reaches
// zero.
//
// If the internal counter goes negative Done will panic.
func (wg *WaitGroup) Done() {
	wg.m.Lock()
	defer wg.m.Unlock()

	wg.count--
	wg.testAndBroadcast()
}

// Wait blocks until internal counter reaches zero.
func (wg *WaitGroup) Wait() {
	wg.m.Lock()
	defer wg.m.Unlock()

	// Set the lock on cond, if not set yet
	if wg.c.L == nil {
		wg.c.L = &wg.m
	}

	// Wait for count to reach zero
	for wg.count != 0 {
		wg.c.Wait()
	}
}

// Drain prevents additional increments using Add(delta)
func (wg *WaitGroup) Drain() {
	wg.m.Lock()
	defer wg.m.Unlock()

	wg.draining = true
}

// WaitAndDrain will wait for the internal counter to reach zero and atomically
// switch to draining mode, so additional Add() calls will fail.
func (wg *WaitGroup) WaitAndDrain() {
	wg.m.Lock()
	defer wg.m.Unlock()

	// Set the lock on cond, if not set yet
	if wg.c.L == nil {
		wg.c.L = &wg.m
	}

	// Wait for count to reach zero
	for wg.count != 0 {
		wg.c.Wait()
	}

	// Set draining
	wg.draining = true
}
