package lifecyclepolicy

import "time"

// A LifeCyclePolicy implements the logic for when to stop a worker.
type LifeCyclePolicy interface {
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

// Base provides a base implemetation of LifeCyclePolicy, implementors should
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
