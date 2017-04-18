package worker

import (
	"context"
	"time"

	"github.com/taskcluster/taskcluster-worker/runtime"
)

// lifeCycleContext implements context.Context such that it is canceled, when
// the given life-cycle tracker ends.
type lifeCycleContext struct {
	LifeCycle *runtime.LifeCycleTracker
}

func (c *lifeCycleContext) Deadline() (deadline time.Time, ok bool) {
	return time.Time{}, false
}

func (c *lifeCycleContext) Done() <-chan struct{} {
	return c.LifeCycle.StoppingGracefully.Done()
}

func (c *lifeCycleContext) Err() error {
	if c.LifeCycle.StoppingGracefully.IsDone() {
		return context.Canceled
	}
	return nil
}

func (c *lifeCycleContext) Value(key interface{}) interface{} {
	return nil
}
