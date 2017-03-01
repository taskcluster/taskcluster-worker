package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-client-go/queue"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

// The taskFlow structure stores state that carried between phases.
// the runTask function uses this structure to ensure execution of all phases.
// The taskFlow structure should not be referenced outside this file.
//
// Notable benefit is that the runTask function handles all the concurrency
// concerns, reclaiming, waiting for worker-shutdown and handling errors.
// So methods implementing phases in taskFlow need not worry about locking.
type taskFlow struct {
	// Constants
	environment runtime.Environment
	engine      engines.Engine
	plugin      plugins.Plugin
	monitor     runtime.Monitor
	task        *queue.TaskDefinitionResponse

	// TaskContext may be canceled when reason have been set
	taskContext *runtime.TaskContext
	controller  *runtime.TaskContextController

	// Flow state to be discarded at end if not nil
	taskPlugin     plugins.TaskPlugin
	sandboxBuilder engines.SandboxBuilder
	sandbox        engines.Sandbox
	resultSet      engines.ResultSet
	success        bool
	reason         runtime.ExceptionReason
}

// runTask will run the task, aborting with worker-shutdown if ctx is canceled
func runTask(
	ctx context.Context,
	claim taskClaim,
	environment runtime.Environment,
	engine engines.Engine,
	plugin plugins.Plugin,
	monitor runtime.Monitor,
	reclaimOffset int,
	queueBaseURL string,
) error {
	// Create TaskContext and controller
	taskContext, controller, err := runtime.NewTaskContext(
		environment.TemporaryStorage.NewFilePath(),
		runtime.TaskInfo{
			TaskID:   claim.taskClaim.Status.TaskID,
			RunID:    claim.taskClaim.RunID,
			Created:  claim.taskClaim.Task.Created,
			Deadline: claim.taskClaim.Task.Deadline,
			Expires:  claim.taskClaim.Task.Expires,
		}, environment.WebHookServer,
	)
	_ = err

	// Create a taskFlow structure for tracking state
	t := taskFlow{
		environment: environment,
		engine:      engine,
		plugin:      plugin,
		monitor:     monitor,
		task:        claim.definition,
		taskContext: taskContext,
		controller:  controller,
		reason:      runtime.ReasonNoException,
	}

	// Reason something was cancelled. We track that here, just like we handle all
	// the currency issues in this function. The methods on run is just
	// responsible for advancing the state, and need not worry about concurrency.
	mReason := sync.Mutex{}
	cancelReason := runtime.ReasonNoException

	cancel := func(reason runtime.ExceptionReason) {
		if reason == runtime.ReasonNoException {
			return
		}
		mReason.Lock()
		if cancelReason == runtime.ReasonNoException {
			cancelReason = reason
		}
		mReason.Unlock()
		controller.Cancel()
	}

	// Wait group to synchronize everything at the end
	wg := sync.WaitGroup{}
	wg.Add(1)
	// Reclaim and wait for workerShutDown in the background
	go func() {
		select {
		case <-ctx.Done():
			cancel(runtime.ReasonWorkerShutdown)
		case <-controller.Done():
		}
		wg.Done()
	}()
	go func() {
		cancel(reclaimForever(controller, queueBaseURL, reclaimOffset))
		wg.Done()
	}()

	/*
		stages := []struct {
			name string
			fn   func(runtime.Monitor) error
		}{
			{"prepare", t.prepare},
			{"build", t.build},
			{"start", t.start},
			{"started", t.started},
			{"waiting", t.waiting},
			{"stopped", t.stopped},
			{"finished", t.finished},
		}
	*/

	// Stages the run has to transition through, each may be followed
	// by resolve(), exception() and dispose(), if an error is returned.
	stages := []func(runtime.Monitor) error{
		t.prepare,
		t.build,
		t.start,
		t.started,
		t.waiting,
		t.stopped,
		t.finished,
	}
	stageNames := []string{ // Names for stages listed above
		"prepare",
		"build",
		"start",
		"started",
		"waiting",
		"stopped",
		"finished",
	}
	for i, stage := range stages {
		t.monitor.Debug("### Stage: ", stageNames[i])
		if err = stage(t.monitor.WithTag("stage", stageNames[i])); err != nil {
			if e, ok := err.(engines.MalformedPayloadError); ok {
				t.controller.LogError("Malformed Payload Error: ", e.Error())
				cancel(runtime.ReasonMalformedPayload)
				break
			}
			t.monitor.WithTags(map[string]string{
				"stage":      stageNames[i],
				"incidentId": "",
				"taskId":     "",
				"runId":      "",
			}).ReportError(err)
			t.controller.LogError("Internal Error, see incidentId: ")
			cancel(runtime.ReasonInternalError)
			break
		}
		if controller.Err() != nil {
			break
		}
	}

	// Get the cancel reason, if any, after this point we can't cancel anymore
	// We're reporting exception, so no matter what happens after this we'll try
	// go through the finalStages
	mReason.Lock()
	t.reason = cancelReason
	mReason.Unlock()

	// Final stages, that always have to go through, we'll return any error that
	// throw, but we won't overwrite any existing error.
	finalStages := []func(runtime.Monitor) error{
		t.resolve,
		t.exceptionIfAny,
		t.dispose,
	}
	for _, stage := range finalStages {
		if err2 := stage(t.monitor); err == nil {
			err = err2
		}
	}

	// Always cancel any context
	controller.Cancel()

	// Wait for all go routines to finish
	wg.Wait()

	// Dispose of TaskContext
	if err2 := controller.Dispose(); err == nil {
		err = err2
	}

	return err
}

func reclaimForever(
	controller *runtime.TaskContextController,
	queueBaseURL string,
	reclaimOffset int,
) runtime.ExceptionReason {
	for {
		select {
		case <-time.After(15 * time.Second):
		case <-controller.Done():
		}
		controller.SetQueueClient(nil)
		return runtime.ReasonInternalError // Or ReasonCanceled...
	}
}

func (t *taskFlow) prepare(monitor runtime.Monitor) error {
	monitor.Debug("### Prepare Stage")

	// Parse payload
	payload := map[string]interface{}{}
	if err := json.Unmarshal([]byte(t.task.Payload), &payload); err != nil {
		return engines.NewMalformedPayloadError("Task payload must be an object")
	}

	// Construct payload schema
	payloadSchema, err := schematypes.Merge(
		t.engine.PayloadSchema(),
		t.plugin.PayloadSchema(),
	)
	if err != nil {
		panic(fmt.Sprintf(
			"Conflicting plugin and engine payload properties, error: %s", err,
		))
	}

	// Validate payload against schema
	if err = payloadSchema.Validate(payload); err != nil {
		return engines.NewMalformedPayloadError("Schema violation: ", err)
	}

	var err1, err2 error
	util.Parallel(func() {
		// Create SandboxBuilder
		t.sandboxBuilder, err1 = t.engine.NewSandboxBuilder(engines.SandboxOptions{
			TaskContext: t.taskContext,
			Payload:     t.engine.PayloadSchema().Filter(payload),
		})
	}, func() {
		// Create TaskPlugin
		t.taskPlugin, err2 = t.plugin.NewTaskPlugin(plugins.TaskPluginOptions{
			TaskInfo: &t.taskContext.TaskInfo,
			Payload:  t.plugin.PayloadSchema().Filter(payload),
			Monitor:  monitor.WithTag("something", "here"),
		})
		if err2 != nil {
			return
		}
		// Prepare TaskPlugin
		err2 = t.taskPlugin.Prepare(t.taskContext)
	})

	// Always prefer to return a MalformedPayloadError, if we have one
	if _, ok := err1.(engines.MalformedPayloadError); ok || err2 == nil {
		return err1
	}
	return err2
}

func (t *taskFlow) build(monitor runtime.Monitor) error {
	monitor.Debug("### Build Stage")
	return t.taskPlugin.BuildSandbox(t.sandboxBuilder)
}

func (t *taskFlow) start(monitor runtime.Monitor) error {
	monitor.Debug("### Start Stage")
	var err error
	t.sandbox, err = t.sandboxBuilder.StartSandbox()
	t.sandboxBuilder = nil
	return err
}

func (t *taskFlow) started(monitor runtime.Monitor) error {
	monitor.Debug("### Started Stage")
	return t.taskPlugin.Started(t.sandbox)
}

func (t *taskFlow) waiting(monitor runtime.Monitor) error {
	monitor.Debug("### Waiting Stage")
	var err error
	t.resultSet, err = t.sandbox.WaitForResult()
	t.sandbox = nil
	return err
}

func (t *taskFlow) stopped(monitor runtime.Monitor) error {
	monitor.Debug("### Stopped Stage")
	var err error
	t.success, err = t.taskPlugin.Stopped(t.resultSet)
	return err
}

func (t *taskFlow) finished(monitor runtime.Monitor) error {
	monitor.Debug("### Finished Stage")

	// Close log
	err := t.controller.CloseLog()
	if err != nil {
		monitor.Error("Failed to close log, err: ", err)
		return err
	}

	// Call finish handler on plugins
	return t.taskPlugin.Finished(t.success)
}

func (t *taskFlow) resolve(monitor runtime.Monitor) error {
	monitor.Debug("### Resolve Stage")

	return nil
}

func (t *taskFlow) exceptionIfAny(monitor runtime.Monitor) error {
	// Skip if there is no error
	if t.reason == runtime.ReasonNoException {
		return nil
	}
	monitor.Debug("### Exception Stage")

	return nil
}

func (t *taskFlow) dispose(monitor runtime.Monitor) error {
	monitor.Debug("### Dispose Stage")

	err := t.taskPlugin.Dispose()
	t.taskPlugin = nil
	_ = err

	err = r.resultSet.Dispose()
	t.resultSet = nil
	_ = err

	return nil
}
