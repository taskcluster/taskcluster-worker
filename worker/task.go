package worker

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/taskcluster/taskcluster-client-go/queue"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

// TaskRun represents the task lifecycle once claimed.  TaskRun contains information
// about the task as well as controllers for executing and resolving the task.
type TaskRun struct {
	TaskID          string                       `json:"taskId"`
	RunID           uint                         `json:"runId"`
	SignedDeleteURL string                       `json:"-"`
	TaskClaim       queue.TaskClaimResponse      `json:"-"`
	TaskReclaim     queue.TaskReclaimResponse    `json:"-"`
	Definition      queue.TaskDefinitionResponse `json:"-"`
	QueueClient     queueClient
	plugin          plugins.TaskPlugin
	log             *logrus.Entry
	payload         interface{}
	pluginPayload   interface{}
	sync.RWMutex
	context        *runtime.TaskContext
	controller     *runtime.TaskContextController
	sandboxBuilder engines.SandboxBuilder
	sandbox        engines.Sandbox
	resultSet      engines.ResultSet
	engine         engines.Engine
	success        bool
}

// Abort will set the status of the task to aborted and abort the task execution
// environment if one has been created.
func (t *TaskRun) Abort() {
	t.Lock()
	defer t.Unlock()
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
	t.Lock()
	defer t.Unlock()
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
// the ExceptionStage.
func (t *TaskRun) Run(pluginManager plugins.Plugin, engine engines.Engine, context *runtime.TaskContext, contextController *runtime.TaskContextController) {
	// TODO (garndt): change to NewRun, return &TaskRun{}, introduce task claim concept into queue service
	t.Lock()
	t.context = context
	t.controller = contextController
	t.engine = engine
	t.Unlock()

	defer t.DisposeStage()

	err := t.ParsePayload(pluginManager, engine)
	if err != nil || t.context.IsAborted() || t.context.IsCancelled() {
		t.context.LogError(fmt.Sprintf("Invalid task payload. %s", err))
		t.ExceptionStage(runtime.Errored, err)
		return
	}

	err = t.CreateTaskPlugins(pluginManager)
	if err != nil || t.context.IsAborted() || t.context.IsCancelled() {
		t.context.LogError(fmt.Sprintf("Invalid task payload. %s", err))
		t.ExceptionStage(runtime.Errored, err)
		return
	}

	stages := []func() error{
		t.PrepareStage,
		t.BuildStage,
		t.StartStage,
		t.StopStage,
		t.FinishStage,
	}
	for _, stage := range stages {
		err = stage()
		if err != nil || t.context.IsAborted() || t.context.IsCancelled() {
			t.ExceptionStage(runtime.Errored, err)
			return
		}
	}

	err = t.ResolveTask()
	if err != nil || t.context.IsAborted() || t.context.IsCancelled() {
		t.log.WithField("error", err.Error()).Warn("Could not resolve task properly")
		return
	}

}

// ParsePayload  will parse the task payload, which will validate it against the engine
// and plugin schemas.
func (t *TaskRun) ParsePayload(pluginManager plugins.Plugin, engine engines.Engine) error {
	var err error
	jsonPayload := map[string]json.RawMessage{}
	if err = json.Unmarshal([]byte(t.Definition.Payload), &jsonPayload); err != nil {
		return err
	}

	t.payload, err = engine.PayloadSchema().Parse(jsonPayload)
	if err != nil {
		return err
	}

	ps, err := pluginManager.PayloadSchema()
	if err != nil {
	}

	t.pluginPayload, err = ps.Parse(jsonPayload)
	if err != nil {
		return err
	}
	return nil
}

// CreateTaskPlugins will create a new task plugin to be used during the task lifecycle.
func (t *TaskRun) CreateTaskPlugins(pluginManager plugins.Plugin) error {
	var err error
	popts := plugins.TaskPluginOptions{TaskInfo: &runtime.TaskInfo{}, Payload: t.pluginPayload}
	t.plugin, err = pluginManager.NewTaskPlugin(popts)
	if err != nil {
		return err
	}

	return nil
}

// PrepareStage is where task plugins are prepared and a sandboxbuilder is created.
func (t *TaskRun) PrepareStage() error {
	t.log.Debug("Preparing task run")

	err := t.plugin.Prepare(t.context)
	if err != nil {
		t.context.LogError(fmt.Sprintf("Could not prepare task plugins. %s", err))
		return err
	}

	t.Lock()
	t.sandboxBuilder, err = t.engine.NewSandboxBuilder(engines.SandboxOptions{
		TaskContext: t.context,
		Payload:     t.payload,
	})
	t.Unlock()
	if err != nil {
		t.context.LogError(fmt.Sprintf("Could not create task execution environment. %s", err))
		return err
	}

	return nil
}

// BuildStage is responsible for configuring the environment for building a sandbox (task execution environment).
func (t *TaskRun) BuildStage() error {
	t.log.Debug("Building task run")

	err := t.plugin.BuildSandbox(t.sandboxBuilder)
	if err != nil {
		t.context.LogError(fmt.Sprintf("Could not create task execution environment. %s", err))
		return err
	}

	return nil
}

// StartStage is responsible for starting the execution environment and waiting for a result.
func (t *TaskRun) StartStage() error {
	t.log.Debug("Running task")

	sandbox, err := t.sandboxBuilder.StartSandbox()
	if err != nil {
		t.context.LogError(fmt.Sprintf("Could not start task execution environment. %s", err))
		return err
	}
	t.Lock()
	t.sandbox = sandbox
	t.Unlock()

	err = t.plugin.Started(t.sandbox)
	if err != nil {
		t.context.LogError(fmt.Sprintf("Could not start task execution environment. %s", err))
		return err
	}

	result, err := t.sandbox.WaitForResult()
	if err != nil {
		t.context.LogError(fmt.Sprintf("Task execution did not complete successfully. %s", err))
		return err
	}

	t.resultSet = result

	return nil
}

// StopStage will run once the sandbox has terminated.  This stage will be responsible
// for uploading artifacts, cleaning up of resources, etc.
func (t *TaskRun) StopStage() error {
	t.log.Debug("Stopping task execution")
	var err error
	t.success, err = t.plugin.Stopped(t.resultSet)

	if err != nil {
		t.context.LogError(fmt.Sprintf("Stopping execution environment failed. %s", err))
		return err
	}

	return nil
}

// FinishStage is responsible for finalizing the execution of a task, close and
// upload tasks logs, etc.
func (t *TaskRun) FinishStage() error {
	t.log.Debug("Finishing task run")

	err := t.controller.CloseLog()
	if err != nil {
		t.log.WithField("error", err.Error()).Warn("Could not properly close task log")
	}

	err = t.plugin.Finished(t.success)
	if err != nil {
		t.context.LogError(fmt.Sprintf("Could not finish cleaning up task execution. %s", err))
		return err
	}

	return nil
}

// ExceptionStage will report a task run as an exception with an appropriate reason.
// Tasks that have been cancelled will not be reported as an exception as the run
// has already been resolved.
func (t *TaskRun) ExceptionStage(status runtime.TaskStatus, taskError error) {
	fmt.Println(taskError)
	var reason runtime.ExceptionReason
	switch taskError.(type) {
	case engines.MalformedPayloadError:
		reason = runtime.MalformedPayload
	case engines.InternalError:
		reason = runtime.InternalError
	default:
		reason = runtime.WorkerShutdown
	}

	err := t.controller.CloseLog()
	if err != nil {
		t.log.WithField("error", err.Error()).Warn("Could not properly close task log")
	}

	// TODO (garndt): handle when task plugins haven't been created yet
	err = t.plugin.Exception(reason)
	if err != nil {
		t.log.WithField("error", err.Error()).Warn("Could not finalize task plugins as exception.")
	}

	if t.context.IsCancelled() {
		return
	}

	e := reportException(t.QueueClient, t, reason, t.log)
	if e != nil {
		t.log.WithField("error", e.Error()).Warn("Could not resolve task as exception.")
	}

	return
}

// ResolveTask will resolve the task as completed/failed depending on the outcome
// of executing the task and finalizing the task plugins.
func (t *TaskRun) ResolveTask() error {
	resolve := reportCompleted
	if !t.success {
		resolve = reportFailed
	}

	err := resolve(t.QueueClient, t, t.log)
	if err != nil {
		return errors.New(err.Error())
	}
	return nil
}

// DisposeStage is responsible for cleaning up resources allocated for the task execution.
// This will involve closing all necessary files and disposing of contexts, plugins, and sandboxes.
func (t *TaskRun) DisposeStage() {
	err := t.controller.Dispose()
	if err != nil {
		t.log.WithField("error", err.Error()).Warn("Could not dispose of task context")
	}
	return
}
