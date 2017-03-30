package worker

import (
	"sync"
	"time"

	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
)

// taskCounter keeps count of number of active tasks as well as idle time.
type taskCounter struct {
	m         sync.Mutex
	c         sync.Cond
	value     int
	idleTimer atomics.StopWatch
}

// initialize the taskCounter, if not already initialized
func (c *taskCounter) init() {
	c.m.Lock()
	defer c.m.Unlock()

	if c.c.L == nil {
		c.c.L = &c.m
		c.idleTimer.Start()
	}
}

// IdleTime returns the duration since last time the worker was working on a
// task. Zero if the worker is currently working.
func (c *taskCounter) IdleTime() time.Duration {
	c.init()
	return c.idleTimer.Elapsed()
}

// WaitForIdle blocks until the active task count is zero
func (c *taskCounter) WaitForIdle() {
	c.init()
	c.m.Lock()
	defer c.m.Unlock()

	for c.value > 0 {
		c.c.Wait()
	}
}

// WaitForLessThan blocks until the active task count is less than given value
func (c *taskCounter) WaitForLessThan(value int) {
	c.init()
	c.m.Lock()
	defer c.m.Unlock()

	for c.value >= value {
		c.c.Wait()
	}
}

// Increment the active task count
func (c *taskCounter) Increment() {
	c.init()
	c.m.Lock()
	defer c.m.Unlock()

	c.value++
	c.idleTimer.Reset()
	c.c.Broadcast()
}

// Decrement the active task count
func (c *taskCounter) Decrement() {
	c.init()
	c.m.Lock()
	defer c.m.Unlock()

	c.value--
	if c.value == 0 {
		c.idleTimer.Start()
	}
	c.c.Broadcast()
	if c.value < 0 {
		panic("worker.taskCounter should never be able to go below zero")
	}
}

// Value returns the number active tasks
func (c *taskCounter) Value() int {
	c.init()
	c.m.Lock()
	defer c.m.Unlock()

	return c.value
}
