// Package taskmgr is responsible for managing the claiming, executing, and resolution
// of tasks.
// This package consists of two key components:

// Task Manager - this component is responsible for the claiming, executing and resolution
// of tasks.  It also is responsible for cancelling tasks that run beyond their max runtime as
// well as for when a cancellation action was triggered by an external entity.
//
// Queue Service - The queue service will retrieve and attempt to claim as many tasks
// as specified from the Azure task queues.  The service will attempt to poll from the highest
// priority queues first, followed by lower priority queues until as many tasks can be claimed
// up to the limit set.

package taskmgr
