package worker

import (
	"sync"
	"time"
)

// taskCounter keeps count of number of active tasks as well as idle time.
type taskCounter struct {
	m         sync.Mutex
	c         sync.Cond
	value     int
	idleSince time.Time
}

// initialize the taskCounter and lock it, if not already initialized
func (c *taskCounter) initAndLock() {
	c.m.Lock()

	if c.c.L == nil {
		c.c.L = &c.m
		c.idleSince = time.Now()
	}
}

// IdleTime returns the duration since last time the worker was working on a
// task. Zero if the worker is currently working.
func (c *taskCounter) IdleTime() time.Duration {
	c.initAndLock()
	defer c.m.Unlock()

	// if task counter is higher than zero, then we're not idle
	if c.value > 0 {
		return 0
	}
	return time.Since(c.idleSince)
}

// WaitForIdle blocks until the active task count is zero
func (c *taskCounter) WaitForIdle() {
	c.initAndLock()
	defer c.m.Unlock()

	for c.value > 0 {
		c.c.Wait()
	}
}

// WaitForLessThan blocks until the active task count is less than given value
func (c *taskCounter) WaitForLessThan(value int) {
	c.initAndLock()
	defer c.m.Unlock()

	for c.value >= value {
		c.c.Wait()
	}
}

// Increment the active task count
func (c *taskCounter) Increment() {
	c.initAndLock()
	defer c.m.Unlock()

	c.value++
	c.c.Broadcast()
}

// Decrement the active task count
func (c *taskCounter) Decrement() {
	c.initAndLock()
	defer c.m.Unlock()

	c.value--
	c.idleSince = time.Now()
	c.c.Broadcast()
	if c.value < 0 {
		panic("worker.taskCounter should never be able to go below zero")
	}
}

// Value returns the number active tasks
func (c *taskCounter) Value() int {
	c.initAndLock()
	defer c.m.Unlock()

	return c.value
}
