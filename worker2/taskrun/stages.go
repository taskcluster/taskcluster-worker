package taskrun

import (
	"fmt"
	"strconv"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

// A Stage represents the internal state at which a TaskRun has been advanced.
type Stage int

// Stages supported by RunStage
const (
	StagePrepare Stage = iota
	StageBuild
	StageStart
	StageStarted
	StageWaiting
	StageStopped
	StageFinished
	stageResolved
)

func (s Stage) String() string {
	switch s {
	case StagePrepare:
		return "prepare"
	case StageBuild:
		return "build"
	case StageStart:
		return "start"
	case StageStarted:
		return "started"
	case StageWaiting:
		return "waiting"
	case StageStopped:
		return "stopped"
	case StageFinished:
		return "finished"
	}
	panic(fmt.Sprintf("Unknown stage '%d' in stage.String()", s))
}

var stages = map[Stage]func(*TaskRun) error{
	StagePrepare:  prepare,
	StageBuild:    build,
	StageStart:    start,
	StageStarted:  started,
	StageWaiting:  waiting,
	StageStopped:  stopped,
	StageFinished: finished,
}

func prepare(t *TaskRun) error {
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
	if err = payloadSchema.Validate(t.payload); err != nil {
		return runtime.NewMalformedPayloadError("Payload schema violation: ", err)
	}

	var err1, err2 error
	util.Parallel(func() {
		// Create SandboxBuilder
		t.sandboxBuilder, err1 = t.engine.NewSandboxBuilder(engines.SandboxOptions{
			TaskContext: t.taskContext,
			Payload:     t.engine.PayloadSchema().Filter(t.payload),
			Monitor: t.environment.Monitor.WithPrefix("engine").WithTags(map[string]string{
				"taskId": t.taskInfo.TaskID,
				"runId":  strconv.Itoa(t.taskInfo.RunID),
			}),
		})
	}, func() {
		// Create TaskPlugin
		t.taskPlugin, err2 = t.plugin.NewTaskPlugin(plugins.TaskPluginOptions{
			TaskInfo:    &t.taskInfo,
			TaskContext: t.taskContext,
			Payload:     t.plugin.PayloadSchema().Filter(t.payload),
			Monitor: t.environment.Monitor.WithPrefix("plugin").WithTags(map[string]string{
				"taskId": t.taskInfo.TaskID,
				"runId":  strconv.Itoa(t.taskInfo.RunID),
			}),
		})
		if err2 != nil {
			return
		}
	})

	// Always prefer to return a MalformedPayloadError, if we have one
	if _, ok := runtime.IsMalformedPayloadError(err1); ok || err2 == nil {
		return err1
	}
	return err2
}

func build(t *TaskRun) error {
	return t.taskPlugin.BuildSandbox(t.sandboxBuilder)
}

func start(t *TaskRun) error {
	var err error
	t.sandbox, err = t.sandboxBuilder.StartSandbox()
	t.sandboxBuilder = nil
	return err
}

func started(t *TaskRun) error {
	return t.taskPlugin.Started(t.sandbox)
}

func waiting(t *TaskRun) error {
	var err error
	t.resultSet, err = t.sandbox.WaitForResult()
	t.sandbox = nil
	return err
}

func stopped(t *TaskRun) error {
	var err error
	t.success, err = t.taskPlugin.Stopped(t.resultSet)
	return err
}

func finished(t *TaskRun) error {
	// Close log
	err := t.controller.CloseLog()
	if err != nil {
		panic(fmt.Sprintf("Failed to close task-log, error: %s", err))
	}

	// Call finish handler on plugins
	return t.taskPlugin.Finished(t.success)
}
