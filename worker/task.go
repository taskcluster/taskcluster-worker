package worker

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-client-go/queue"
	"github.com/taskcluster/taskcluster-client-go/tcclient"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

// TaskRun represents the task lifecycle once claimed.  TaskRun contains information
// about the task as well as controllers for executing and resolving the task.
type TaskRun struct {
	// Static properties
	TaskID     string
	RunID      int
	definition *queue.TaskDefinitionResponse
	plugin     plugins.Plugin
	log        *logrus.Entry
	// Internal state
	m              sync.Mutex
	taskPlugin     plugins.TaskPlugin
	context        *runtime.TaskContext
	controller     *runtime.TaskContextController
	sandboxBuilder engines.SandboxBuilder
	sandbox        engines.Sandbox
	resultSet      engines.ResultSet
	engine         engines.Engine
	success        bool
	shutdown       bool
	// Reclaim logic
	queueURL     string
	stopReclaims chan struct{} // close to stop reclaiming task
	reclaimsDone chan struct{} // closed when we have stopped reclaiming the task
}

func newTaskRun(
	config *configType,
	claim *taskClaim,
	environment *runtime.Environment,
	engine engines.Engine,
	plugin plugins.Plugin,
	log *logrus.Entry,
) (*TaskRun, error) {

	tp := environment.TemporaryStorage.NewFilePath()
	info := runtime.TaskInfo{
		TaskID:   claim.taskClaim.Status.TaskID,
		RunID:    claim.taskClaim.RunID,
		Created:  claim.taskClaim.Task.Created,
		Deadline: claim.taskClaim.Task.Deadline,
		Expires:  claim.taskClaim.Task.Expires,
	}
	ctxt, ctxtctl, err := runtime.NewTaskContext(tp, info, environment.WebHookServer)

	queueClient := queue.New(&tcclient.Credentials{
		ClientID:    claim.taskClaim.Credentials.ClientID,
		AccessToken: claim.taskClaim.Credentials.AccessToken,
		Certificate: claim.taskClaim.Credentials.Certificate,
	})

	if config.QueueBaseURL != "" {
		queueClient.BaseURL = config.QueueBaseURL
	}

	ctxtctl.SetQueueClient(queueClient)

	if err != nil {
		return nil, err
	}

	t := &TaskRun{
		TaskID:       claim.taskID,
		RunID:        claim.runID,
		definition:   claim.definition,
		log:          log,
		context:      ctxt,
		controller:   ctxtctl,
		engine:       engine,
		plugin:       plugin,
		queueURL:     config.QueueBaseURL,
		stopReclaims: make(chan struct{}),
		reclaimsDone: make(chan struct{}),
	}

	go t.reclaim(time.Time(claim.taskClaim.TakenUntil))

	return t, nil
}

func (t *TaskRun) reclaim(until time.Time) {
	for {
		duration := until.Sub(time.Now()).Seconds()
		// Using a reclaim divisor of 1.3 with the default reclaim deadline (20 minutes),
		// means that a reclaim event will happen with a few minutes left of the origin claim.
		nextReclaim := duration / 1.3
		select {
		case <-t.stopReclaims:
			close(t.reclaimsDone)
			return
		case <-time.After(time.Duration(nextReclaim * 1e+9)):
			client := t.controller.Queue()
			claim, err := reclaimTask(client, t.TaskID, t.RunID, t.log)
			if err != nil {
				t.log.WithError(err).Error("Error reclaiming task")
				t.Abort()
				close(t.reclaimsDone)
				return
			}

			queueClient := queue.New(&tcclient.Credentials{
				ClientID:    claim.Credentials.ClientID,
				AccessToken: claim.Credentials.AccessToken,
				Certificate: claim.Credentials.Certificate,
			})

			queueClient.BaseURL = t.queueURL
			t.controller.SetQueueClient(queueClient)
			until = time.Time(claim.TakenUntil)
		}
	}
}

// Abort will set the status of the task to aborted and abort the task execution
// environment if one has been created.
func (t *TaskRun) Abort() {
	t.m.Lock()
	defer t.m.Unlock()

	t.shutdown = true
	if t.context != nil {
		t.context.Abort()
	}
	if t.sandbox != nil {
		t.sandbox.Abort()
	}
}

// Cancel will set the status of the task to cancelled and abort the task execution
// environment if one has been created.
func (t *TaskRun) Cancel() {
	t.log.Info("Cancelling task")
	t.m.Lock()
	defer t.m.Unlock()

	if t.context != nil {
		t.context.Cancel()
	}
	if t.sandbox != nil {
		t.sandbox.Abort()
	}
}

// Run is the entrypoint to executing a task run.  The task payload will be parsed,
// plugins created, and the task will run through each of the stages of the task
// life cycle.
//
// Tasks that do not complete successfully will be reported as an exception during
// the exceptionStage.
func (t *TaskRun) Run() {
	defer t.disposeStage()

	stages := []func() error{
		t.prepareStage,
		t.buildStage,
		t.startStage,
		t.stopStage,
		t.finishStage,
	}

	for _, stage := range stages {
		err := stage()
		if err != nil || t.context.IsAborted() || t.context.IsCancelled() {
			t.exceptionStage(err)
			return
		}
	}

	err := t.resolveTask()
	if err != nil || t.context.IsAborted() || t.context.IsCancelled() {
		t.log.WithField("error", err.Error()).Warn("Could not resolve task properly")
		return
	}

}

// prepareStage is where task plugins are prepared and a sandboxbuilder is created.
func (t *TaskRun) prepareStage() error {
	t.log.Debug("Preparing task run")

	// Parse payload
	payload := map[string]interface{}{}
	err := json.Unmarshal([]byte(t.definition.Payload), &payload)
	if err != nil {
		err = engines.NewMalformedPayloadError(fmt.Sprintf("Invalid task payload. %s", err))
		t.context.LogError(err.Error())
		return err
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
	err = payloadSchema.Validate(payload)
	if err != nil {
		err = engines.NewMalformedPayloadError("Schema violation: ", err)
		t.context.LogError(err.Error())
		return err
	}

	// Create TaskPlugin
	t.taskPlugin, err = t.plugin.NewTaskPlugin(plugins.TaskPluginOptions{
		TaskInfo: &runtime.TaskInfo{
			TaskID: t.TaskID,
			RunID:  t.RunID,
		},
		Payload: t.plugin.PayloadSchema().Filter(payload),
	})
	if err != nil {
		t.context.LogError(err.Error())
		return err
	}

	// Prepare TaskPlugin
	err = t.taskPlugin.Prepare(t.context)
	if err != nil {
		t.context.LogError(fmt.Sprintf("Could not prepare task plugins. %s", err))
		return err
	}

	sandboxBuilder, err := t.engine.NewSandboxBuilder(engines.SandboxOptions{
		TaskContext: t.context,
		Payload:     t.engine.PayloadSchema().Filter(payload),
	})
	t.m.Lock()
	t.sandboxBuilder = sandboxBuilder
	t.m.Unlock()
	if err != nil {
		t.context.LogError(fmt.Sprintf("Could not create task execution environment. %s", err))
		return err
	}

	return nil
}

// buildStage is responsible for configuring the environment for building a sandbox (task execution environment).
func (t *TaskRun) buildStage() error {
	t.log.Debug("Building task run")

	err := t.taskPlugin.BuildSandbox(t.sandboxBuilder)
	if err != nil {
		t.context.LogError(fmt.Sprintf("Could not create task execution environment. %s", err))
		return err
	}

	return nil
}

// startStage is responsible for starting the execution environment and waiting for a result.
func (t *TaskRun) startStage() error {
	t.log.Debug("Running task")

	sandbox, err := t.sandboxBuilder.StartSandbox()
	if err != nil {
		t.context.LogError(fmt.Sprintf("Could not start task execution environment. %s", err))
		return err
	}
	t.m.Lock()
	t.sandbox = sandbox
	t.m.Unlock()

	err = t.taskPlugin.Started(t.sandbox)
	if err != nil {
		t.context.LogError(fmt.Sprintf("Could not start task execution environment. %s", err))
		return err
	}

	result, err := t.sandbox.WaitForResult()
	if err != nil {
		t.context.LogError(fmt.Sprintf("Task execution did not complete successfully. %s", err))
		return err
	}

	t.m.Lock()
	t.resultSet = result
	t.m.Unlock()

	return nil
}

// stopStage will run once the sandbox has terminated.  This stage will be responsible
// for uploading artifacts, cleaning up of resources, etc.
func (t *TaskRun) stopStage() error {
	t.log.Debug("Stopping task execution")
	var err error
	t.success, err = t.taskPlugin.Stopped(t.resultSet)

	if err != nil {
		t.context.LogError(fmt.Sprintf("Stopping execution environment failed. %s", err))
		return err
	}

	return nil
}

// finishStage is responsible for finalizing the execution of a task, close and
// upload tasks logs, etc.
func (t *TaskRun) finishStage() error {
	t.log.Debug("Finishing task run")

	err := t.controller.CloseLog()
	if err != nil {
		t.log.WithField("error", err.Error()).Warn("Could not properly close task log")
	}

	err = t.taskPlugin.Finished(t.success)
	if err != nil {
		t.log.Error(fmt.Sprintf("Could not finish cleaning up task execution. %s", err))
		return err
	}

	return nil
}

// exceptionStage will report a task run as an exception with an appropriate reason.
// Tasks that have been cancelled will not be reported as an exception as the run
// has already been resolved.
func (t *TaskRun) exceptionStage(taskError error) {
	close(t.stopReclaims)
	<-t.reclaimsDone
	var reason runtime.ExceptionReason
	if t.shutdown {
		reason = runtime.WorkerShutdown
	} else {
		switch taskError.(type) {
		case engines.MalformedPayloadError:
			reason = runtime.MalformedPayload
		default:
			reason = runtime.InternalError
		}
	}

	err := t.controller.CloseLog()
	if err != nil {
		t.log.WithField("error", err.Error()).Warn("Could not properly close task log")
	}

	if t.taskPlugin != nil {
		err = t.taskPlugin.Exception(reason)
		if err != nil {
			t.log.WithField("error", err.Error()).Warn("Could not finalize task plugins as exception.")
		}
	}

	if t.context.IsCancelled() {
		return
	}

	e := reportException(t.context.Queue(), t, reason, t.log)
	if e != nil {
		t.log.WithField("error", e.Error()).Warn("Could not resolve task as exception.")
	}

	return
}

// resolveTask will resolve the task as completed/failed depending on the outcome
// of executing the task and finalizing the task plugins.
func (t *TaskRun) resolveTask() error {
	close(t.stopReclaims)
	<-t.reclaimsDone
	resolve := reportCompleted
	if !t.resultSet.Success() {
		resolve = reportFailed
	}

	err := resolve(t.context.Queue(), t, t.log)
	if err != nil {
		return errors.New(err.Error())
	}
	return nil
}

// disposeStage is responsible for cleaning up resources allocated for the task execution.
// This will involve closing all necessary files and disposing of contexts, plugins, and sandboxes.
func (t *TaskRun) disposeStage() {
	if t.taskPlugin != nil {
		err := t.taskPlugin.Dispose()
		if err != nil {
			t.log.WithError(err).Warn("Could not dispose plugin")
		}
	}

	if t.resultSet != nil {
		err := t.resultSet.Dispose()
		if err != nil {
			t.log.WithError(err).Warn("Could not dispose result set")
		}
	}

	err := t.controller.Dispose()
	if err != nil {
		t.log.WithError(err).Warn("Could not dispose of task context")
	}

	return
}
