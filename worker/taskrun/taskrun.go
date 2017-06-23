package taskrun

import (
	"errors"
	"fmt"
	"sync"

	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
)

// A TaskRun holds the state of a running task.
//
// Methods on this object is not thread-safe, with the exception of Abort() and
// SetQueueClient() which are intended to be called from other threads.
type TaskRun struct {
	// Constants
	environment   runtime.Environment
	engine        engines.Engine
	pluginManager plugins.Plugin // use Plugin interface so we can mock it in tests
	monitor       runtime.Monitor
	taskInfo      runtime.TaskInfo
	payload       map[string]interface{}

	// TaskContext
	taskContext *runtime.TaskContext
	controller  *runtime.TaskContextController

	// State
	m         sync.Mutex // lock protecting state variables
	c         sync.Cond  // Broadcast when state changes
	running   bool       // true, when a thread is advancing the stage
	stage     Stage      // next stage to be run
	success   bool       // true, if task is completed successfully
	exception bool       // true, if reason has a value
	reason    runtime.ExceptionReason

	// Final error to return from Dispose()
	fatalErr    atomics.Bool // If we've seen ErrFatalInternalError
	nonFatalErr atomics.Bool // If we've seen ErrNonFatalInternalError

	// Flow state to be discarded at end if not nil
	taskPlugin     plugins.TaskPlugin
	sandboxBuilder engines.SandboxBuilder
	sandbox        engines.Sandbox
	resultSet      engines.ResultSet
}

// New returns a new TaskRun
func New(options Options) *TaskRun {
	// simple validation, having this a few places is just sane
	options.mustBeValid()

	t := &TaskRun{
		environment:   options.Environment,
		engine:        options.Engine,
		pluginManager: options.PluginManager,
		monitor:       options.Monitor,
		taskInfo:      options.TaskInfo,
		payload:       options.Payload,
	}
	t.c.L = &t.m

	// Create TaskContext and controller
	var err error
	t.taskContext, t.controller, err = runtime.NewTaskContext(
		t.environment.TemporaryStorage.NewFilePath(),
		t.taskInfo,
	)
	if err != nil {
		t.monitor.WithTag("stage", "init").ReportError(err, "failed to create TaskContext")
		// If we have an initial error, we just set state as resolved
		t.stage = stageResolved
		t.success = false
		t.exception = true
		t.reason = runtime.ReasonInternalError
		t.fatalErr.Set(true)
	} else {
		t.controller.SetQueueClient(options.Queue)
	}
	return t
}

// SetQueueClient will update the queue client exposed through the TaskContext.
//
// This should be updated whenever the task is reclaimed.
func (t *TaskRun) SetQueueClient(queue client.Queue) {
	if t.controller != nil {
		t.controller.SetQueueClient(queue)
	}
}

// SetCredentials is used to provide the task-specific temporary credentials,
// and update these whenever they change.
func (t *TaskRun) SetCredentials(clientID, accessToken, certificate string) {
	if t.controller != nil {
		t.controller.SetCredentials(clientID, accessToken, certificate)
	}
}

// Abort will interrupt task execution.
func (t *TaskRun) Abort(reason AbortReason) {
	t.m.Lock()
	defer t.m.Unlock()

	// If we are already resolved, we won't change the resolution
	if t.stage == stageResolved {
		debug("ignoring TaskRun.Abort() as TaskRun is resolved")
		return
	}

	// Resolve this taskrun
	t.stage = stageResolved
	t.success = false
	t.exception = true

	// Set reason we are canceled
	switch reason {
	case WorkerShutdown:
		t.reason = runtime.ReasonWorkerShutdown
	case TaskCanceled:
		t.reason = runtime.ReasonCanceled
	default:
		panic(fmt.Sprintf("Unknown AbortReason: %d", reason))
	}
	// Abort anything that's currently running
	t.controller.Cancel()

	// Inform anyone waiting for resolution
	t.c.Broadcast()
}

// RunToStage will run all stages up-to and including the given stage.
//
// This will not rerun previous stages, the TaskRun structure always knows what
// stage it has executed. This is only useful for testing, the WaitForResult()
// method will run all stages before returning.
func (t *TaskRun) RunToStage(targetStage Stage) {
	t.m.Lock()
	defer t.m.Unlock()

	// Validate input for santiy
	if targetStage > StageFinished {
		panic("RunToStage: stage > StageFinished is not allowed")
	}

	// We'll have no more than on thread running stages at any given time, so we
	// wait till running is false
	for t.running {
		// if t.stage has advanced beyond stage, then we're done
		if t.stage > targetStage {
			return
		}
		t.c.Wait() // wait for state change
	}

	t.running = true // set running while we're inside the for-loop
	for t.stage <= targetStage {

		// Unlock so cancel can happen while we're running
		stage := t.stage // take stage first, so we don't race
		t.m.Unlock()
		monitor := t.monitor.WithTag("stage", stage.String())
		monitor.Debug("running stage: ", stage.String())
		var err error
		incidentID := monitor.CapturePanic(func() {
			err = stages[stage](t)
		})
		t.m.Lock()

		// Handle errors
		if err != nil || incidentID != "" {
			reason := runtime.ReasonInternalError
			if e, ok := runtime.IsMalformedPayloadError(err); ok {
				for _, m := range e.Messages() {
					t.controller.LogError(m)
				}
				reason = runtime.ReasonMalformedPayload
			} else if err == runtime.ErrNonFatalInternalError {
				t.nonFatalErr.Set(true)
			} else if err == runtime.ErrFatalInternalError {
				t.fatalErr.Set(true)
			} else if err != nil {
				incidentID = monitor.ReportError(err)
			}
			if incidentID != "" {
				t.fatalErr.Set(true)
				t.controller.LogError("Unhandled worker error encountered incidentID=", incidentID)
			}
			// Never change the resolution, if we've been cancelled or worker-shutdown
			if t.stage != stageResolved {
				t.stage = stageResolved
				t.exception = true
				t.success = false
				t.reason = reason
			}
		}

		// Never advance beyond stageResolved (could otherwise happen if cancel occurs)
		if t.stage < stageResolved {
			t.stage++
		}
		t.c.Broadcast()
	}

	// if resolved we always cancel the TaskContext
	if t.stage == stageResolved {
		t.controller.Cancel()
	}

	t.running = false
	t.c.Broadcast()
}

// WaitForResult will run all stages up to and including StageFinished, before
// returning the resolution of the given TaskRun.
func (t *TaskRun) WaitForResult() (success bool, exception bool, reason runtime.ExceptionReason) {
	t.RunToStage(StageFinished)

	t.m.Lock()
	success = t.success
	exception = t.exception
	reason = t.reason
	t.m.Unlock()

	return
}

func (t *TaskRun) capturePanicAndError(stage string, fn func() error) {
	monitor := t.monitor.WithTag("stage", stage)
	var err error
	incidentID := monitor.CapturePanic(func() {
		err = fn()
	})
	if incidentID != "" {
		err = runtime.ErrFatalInternalError
	}
	switch err {
	case runtime.ErrFatalInternalError:
		t.fatalErr.Set(true)
	case runtime.ErrNonFatalInternalError:
		t.nonFatalErr.Set(true)
	case nil:
		return
	default:
		t.fatalErr.Set(true)
		monitor.ReportError(err, "unhandled error in stage: ", stage)
	}
}

// Dispose will finish any final processing dispose of all resources.
//
// If there was an unhandled error Dispose() returns either
// runtime.ErrFatalInternalError or runtime.ErrNonFatalInternalError.
// Any other error is reported/logged and runtime.ErrFatalInternalError is
// returned instead.
func (t *TaskRun) Dispose() error {
	t.monitor.WithTag("stage", "dispose").Debug("running stage: dispose")

	if t.controller != nil {
		debug("canceling TaskContext and closing log")
		t.controller.Cancel()
		t.capturePanicAndError("dispose", t.controller.CloseLog)
	}

	if t.exception && t.taskPlugin != nil {
		debug("running exception stage, reason = %s", t.reason.String())
		t.capturePanicAndError("exception", func() error {
			return t.taskPlugin.Exception(t.reason)
		})
	}

	// Dispose of taskPlugin, if we have one
	if t.taskPlugin != nil {
		debug("disposing TaskPlugin")
		t.capturePanicAndError("dispose", t.taskPlugin.Dispose)
		t.taskPlugin = nil
	}

	if t.sandboxBuilder != nil {
		debug("disposing SandboxBuilder")
		t.capturePanicAndError("dispose", t.sandboxBuilder.Discard)
		t.sandboxBuilder = nil
	}

	if t.sandbox != nil {
		debug("disposing Sandbox")
		t.capturePanicAndError("dispose", func() error {
			err := t.sandbox.Abort()
			if err == engines.ErrSandboxTerminated {
				// If the TaskRun was interrupted before the 'waiting' stage, then the
				// execution may have terminated, in which case calling WaitForResult()
				// should be instant, and we get a ResultSet we'll dispose off later
				err = nil
				if t.resultSet == nil {
					t.resultSet, err = t.sandbox.WaitForResult()
					if err == engines.ErrSandboxAborted {
						err = nil // Ignore the error, we'll warn about the contract violation
						t.monitor.WithTag("stage", "dispose").ReportWarning(errors.New(
							"Received ErrSandboxAborted from WaitForResult() after Abort() reported ErrSandboxTerminated",
						), "contract violation during TaskRun.Dispose()")
					}
				}
			}
			return err
		})
		t.sandbox = nil
	}

	if t.resultSet != nil {
		debug("disposing ResultSet")
		t.capturePanicAndError("dispose", t.resultSet.Dispose)
		t.resultSet = nil
	}

	if t.controller != nil {
		debug("disposing TaskContext")
		t.capturePanicAndError("dispose", t.controller.Dispose)
		t.controller = nil
	}

	// We report any errors, so they'll be in sentry and logs, hence, we just
	// notify caller about the fact that there was an unhandled error.
	if t.fatalErr.Get() {
		return runtime.ErrFatalInternalError
	}
	if t.nonFatalErr.Get() {
		return runtime.ErrNonFatalInternalError
	}
	return nil
}
