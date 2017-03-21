// Package lifecyclepolicy defines the interface that lifecyclepolicy providers
// must satisfy and exposes a plugin register that life-cycle policies must be
// registered with.
//
// A life-cycle policy is given a Stoppable object representing the worker,
// which can be used to stop the worker either gracefully or immediately.
// Further more a LifeCyclePolicy is called when certain events occur such as
// idle time, non-fatal events, tasks claimed, or resolved. A LifeCyclePolicy
// uses these events in addition to internal logic and timers to decide when/if
// the workers should be stopped.
//
// A LifeCyclePolicy can stop the worker gracefully or immediately at any time.
// The LifeCyclePolicy is not responsible for tracking whether or not the worker
// has active tasks, if it wishes the stop gracefully it can call initiate such
// action at any time and the worker is responsible for not claiming new tasks.
package lifecyclepolicy
