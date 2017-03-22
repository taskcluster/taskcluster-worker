package worker

import (
	"sync"
	"time"

	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
)

// taskCounter keeps count of number of active tasks as well as pending time.
type taskCounter struct {
	m         sync.Mutex
	c         sync.Cond
	value     int
	idleTimer atomics.StopWatch
}

func (c *taskCounter) init() {
	c.m.Lock()
	defer c.m.Unlock()

	if c.c.L == nil {
		c.c.L = &c.m
		c.idleTimer.Start()
	}
}

func (c *taskCounter) IdleTime() time.Duration {
	c.init()
	return c.idleTimer.Elapsed()
}

func (c *taskCounter) WaitForIdle() {
	c.init()
	c.m.Lock()
	defer c.m.Unlock()

	for c.value != 0 {
		c.c.Wait()
	}
}

func (c *taskCounter) WaitForCapacity(maxConcurrency int) {
	c.init()
	c.m.Lock()
	defer c.m.Unlock()

	for c.value >= maxConcurrency {
		c.c.Wait()
	}
}

func (c *taskCounter) Increment() {
	c.init()
	c.m.Lock()
	defer c.m.Unlock()

	c.value++
	c.idleTimer.Reset()
}

func (c *taskCounter) Decrement() {
	c.init()
	c.m.Lock()
	defer c.m.Unlock()

	c.value--
	if c.value == 0 {
		c.idleTimer.Start()
	}
	if c.value < 0 {
		panic("worker.taskCounter should never be able to go below zero")
	}
}

func (c *taskCounter) Value() int {
	c.init()
	c.m.Lock()
	defer c.m.Unlock()

	return c.value
}
