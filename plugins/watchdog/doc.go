// Package watchdog provides a taskcluster-worker plugin that pokes a watchdog
// whenever a task makes progress or the worker reports that it's idle.
//
// This serves to ensure liveness of the worker, and guard against livelocks.
// Often, golang will detect if the entire program deadlocks, but we can easily
// have a go routine working in the background while still being deadlocked.
//
// Lack of progress on any task or lack of polling for new tasks when idle is
// a solid indicator that something is stuck in a livelock or deadlock. Often
// times a good solution is to report to sentry and kill the worker. Ideally,
// such bugs can be found and fixed over time.
package watchdog
