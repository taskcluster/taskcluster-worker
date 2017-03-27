package taskrun

// An AbortReason specifies the reason a TaskRun was aborted.
type AbortReason int

const (
	// WorkerShutdown is used to abort a TaskRun because the worker is going to
	// shutdown immediately.
	WorkerShutdown AbortReason = 1 + iota
	// TaskCanceled is used to abort a TaskRun when the queue reports that the
	// task has been canceled, deadline exceeded or claim expired.
	TaskCanceled
)
