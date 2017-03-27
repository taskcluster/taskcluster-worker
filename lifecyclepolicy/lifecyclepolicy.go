package lifecyclepolicy

import (
	"time"

	"github.com/taskcluster/taskcluster-worker/runtime"
)

// A LifeCyclePolicy provides a way to construct a Controller for a Stoppable
// worker.
type LifeCyclePolicy interface {
	NewController(worker runtime.Stoppable) Controller
}

// A Controller implements a life-cycle policy for the worker given when it
// was created in NewController()
type Controller interface {
	// ReportIdle is called when the worker has been idle for some time.
	// The parameter d is the time since the worker was last busy.
	ReportIdle(d time.Duration)

	// ReportTaskClaimed is called when the worker has claimed N task.
	ReportTaskClaimed(N int)

	// ReportTaskResolved is called when the worker has resolved a task.
	// The parameter d is the time it took to resolve the task, notice that
	// multiple tasks may be running at the same time, so this does not add up
	// with the time given in ReportIdle.
	ReportTaskResolved(d time.Duration)

	// ReportNonFatalError is called when the worker has encountered a non-fatal
	// error of some sort..
	ReportNonFatalError()
}

// Base provides a base implemetation of Controller, implementors should
// always embed this to ensure forward compatibility.
type Base struct{}

// ReportIdle does nothing, but serves to provide an empty implementation
func (Base) ReportIdle(time.Duration) {}

// ReportTaskClaimed does nothing, but serves to provide an empty implementation
func (Base) ReportTaskClaimed(int) {}

// ReportTaskResolved does nothing, but serves to provide an empty implementation
func (Base) ReportTaskResolved(time.Duration) {}

// ReportNonFatalError does nothing, but serves to provide an empty implementation
func (Base) ReportNonFatalError() {}
