package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/Sirupsen/logrus"
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
	log         *logrus.Entry
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
	log *logrus.Entry,
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
		log:         log,
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

	// Stages the run has to transition through, each may be followed
	// by resolve(), exception() and dispose(), if an error is returned.
	stages := []func(*logrus.Entry) error{
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
		t.log.Debug("### Stage: ", stageNames[i])
		if err = stage(t.log.WithField("stage", stageNames[i])); err != nil {
			if e, ok := err.(engines.MalformedPayloadError); ok {
				t.controller.LogError("Malformed Payload Error: ", e.Error())
				cancel(runtime.ReasonMalformedPayload)
				break
			}
			t.environment.Sentry.CaptureErrorAndWait(err, map[string]string{
				"stage":      stageNames[i],
				"incidentId": "",
				"taskId":     "",
				"runId":      "",
			})
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
	finalStages := []func(*logrus.Entry) error{
		t.resolve,
		t.exceptionIfAny,
		t.dispose,
	}
	for _, stage := range finalStages {
		if err2 := stage(); err == nil {
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
		<-controller.Done()
		controller.SetQueueClient(nil)
		return runtime.ReasonInternalError // Or ReasonCanceled...
	}
}

func (t *taskFlow) prepare(log *logrus.Entry) error {
	t.log.Debug("### Prepare Stage")

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
			Log:      t.log.WithField("plugin", "pluginManager"),
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

func (t *taskFlow) build(log *logrus.Entry) error {
	t.log.Debug("### Build Stage")
	return t.taskPlugin.BuildSandbox(t.sandboxBuilder)
}

func (t *taskFlow) start(log *logrus.Entry) error {
	t.log.Debug("### Start Stage")
	var err error
	t.sandbox, err = t.sandboxBuilder.StartSandbox()
	t.sandboxBuilder = nil
	return err
}

func (t *taskFlow) started(log *logrus.Entry) error {
	t.log.Debug("### Started Stage")
	return t.taskPlugin.Started(t.sandbox)
}

func (t *taskFlow) waiting(log *logrus.Entry) error {
	t.log.Debug("### Waiting Stage")
	var err error
	t.resultSet, err = t.sandbox.WaitForResult()
	t.sandbox = nil
	return err
}

func (t *taskFlow) stopped(log *logrus.Entry) error {
	t.log.Debug("### Stopped Stage")
	var err error
	t.success, err = t.taskPlugin.Stopped(t.resultSet)
	return err
}

func (t *taskFlow) finished(log *logrus.Entry) error {
	t.log.Debug("### Finished Stage")

	// Close log
	err := t.controller.CloseLog()
	if err != nil {
		t.log.WithError(err).Error("Failed to close log, err: ", err)
		return err
	}

	// Call finish handler on plugins
	return t.taskPlugin.Finished(t.success)
}

func (t *taskFlow) resolve(log *logrus.Entry) error {
	t.log.Debug("### Resolve Stage")

	return nil
}

func (t *taskFlow) exceptionIfAny(log *logrus.Entry) error {
	// Skip if there is no error
	if t.reason == runtime.ReasonNoException {
		return nil
	}
	t.log.Debug("### Exception Stage")

	return nil
}

func (t *taskFlow) dispose(log *logrus.Entry) error {
	t.log.Debug("### Dispose Stage")

	err := t.taskPlugin.Dispose()
	t.taskPlugin = nil
	_ = err

	err = r.resultSet.Dispose()
	t.resultSet = nil
	_ = err

	return nil
}
