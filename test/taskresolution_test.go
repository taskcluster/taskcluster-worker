package test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	tcclient "github.com/taskcluster/taskcluster-client-go"
)

// Test failure should resolve as "failed"
func TestFailureResolvesAsFailure(t *testing.T) {
	task, workerType := NewTestTask("TestFailureResolvesAsFailure")
	payload := TaskPayload{
		Command:    failCommand(),
		MaxRunTime: 10,
	}
	taskID, q := SubmitTask(t, task, payload)
	RunTestWorker(workerType, 1)

	EnsureTaskResolution(t, q, taskID, "failed", "failed")
}

func TestAbortAfterMaxRunTime(t *testing.T) {
	task, workerType := NewTestTask("TestAbortAfterMaxRunTime")
	payload := TaskPayload{
		Command:    sleep(4),
		MaxRunTime: 3,
	}
	taskID, q := SubmitTask(t, task, payload)
	RunTestWorker(workerType, 1)

	EnsureTaskResolution(t, q, taskID, "failed", "failed")
	logtext := LogText(t, q, taskID)
	// check uploaded log mentions abortion
	// note: we do this rather than local log, to check also log got uploaded
	// as failure path requires that task is resolved before logs are uploaded
	if !strings.Contains(logtext, "max run time exceeded") {
		t.Fatalf("Was expecting log file to mention task abortion, but it doesn't")
	}
}

func TestIdleWithoutCrash(t *testing.T) {
	task, workerType := NewTestTask("TestIdleWithoutCrash")
	payload := TaskPayload{
		Command:    helloGoodbye(),
		MaxRunTime: 3,
	}
	// this time we'll run worker for ten seconds before submitting the above
	// task, to check it can run for at least ten seconds without crashing, also
	// when no tasks are in the queue
	done := make(chan int)
	go func() {
		RunTestWorker(workerType, 1)
		close(done)
	}()
	// make sure channel doesn't close within ten seconds => worker died in first
	// ten seconds, even though no task was provided...
	timeout := time.After(10 * time.Second)
	select {
	case <-done:
		t.Fatal("Worker exited unexpectedly")
	case <-timeout:
		t.Log("Worker ran for 7 seconds without crashing")
	}
	// Ten seconds of no tasks is up, let's exit worker by running a trivial task
	// that just writes to standard out and exits.
	taskID, q := SubmitTask(t, task, payload)
	// Let's give worker another ten seconds to run this simple task and exit...
	timeout = time.After(10 * time.Second)
	select {
	case <-done:
		t.Log("Worker terminated as expected")
	case <-timeout:
		t.Fatal("Worker did not terminate within 10 seconds of submitting task")
	}
	// Just to be safe, let's check task resolved successfully...
	EnsureTaskResolution(t, q, taskID, "completed", "completed")
}

// Makes sure that if a running task gets cancelled externally, the worker can
// continue to run other tasks (i.e. it doesn't crash etc)
func TestResolveResolvedTask(t *testing.T) {
	task, workerType := NewTestTask("TestResolveResolvedTask - task 1")
	payload := TaskPayload{
		Command:    resolveTask(),
		MaxRunTime: 60,
	}
	fullCreds := &tcclient.Credentials{
		ClientID:    os.Getenv("TASKCLUSTER_CLIENT_ID"),
		AccessToken: os.Getenv("TASKCLUSTER_ACCESS_TOKEN"),
		Certificate: os.Getenv("TASKCLUSTER_CERTIFICATE"),
	}
	if fullCreds.AccessToken == "" || fullCreds.ClientID == "" || fullCreds.Certificate != "" {
		t.Skip("Skipping TestResolveResolvedTask since I need permanent TC credentials for this test")
	}
	tempCreds, err := fullCreds.CreateNamedTemporaryCredentials("project/taskcluster:taskcluster-worker-tester/TestResolveResolvedTask", time.Minute, "queue:cancel-task:"+task.SchedulerID+"/"+task.TaskGroupID+"/*")
	if err != nil {
		t.Fatalf("%v", err)
	}
	env := map[string]string{
		"TASKCLUSTER_CLIENT_ID":    tempCreds.ClientID,
		"TASKCLUSTER_ACCESS_TOKEN": tempCreds.AccessToken,
		"TASKCLUSTER_CERTIFICATE":  tempCreds.Certificate,
	}
	jsonBytes, err := json.Marshal(&env)
	if err != nil {
		t.Fatalf("%v", err)
	}
	var envRawMessage json.RawMessage
	err = json.Unmarshal(jsonBytes, &envRawMessage)
	if err != nil {
		t.Fatalf("%v", err)
	}
	payload.Env = envRawMessage
	taskID1, q := SubmitTask(t, task, payload)

	// now a boring task, just to check it gets run
	task, _ = NewTestTask("TestResolveResolvedTask - task 2")
	// reset worker type to same as first task, as we will run them in one session
	task.WorkerType = workerType

	payload = TaskPayload{
		Command:    helloGoodbye(),
		MaxRunTime: 3,
	}
	taskID2, _ := SubmitTask(t, task, payload)
	RunTestWorker(workerType, 2)

	EnsureTaskResolution(t, q, taskID1, "exception", "canceled")
	EnsureTaskResolution(t, q, taskID2, "completed", "completed")
}

// Make sure that a malformed payload resolves correctly and uploads a log
func TestMalformedPayload(t *testing.T) {
	task, workerType := NewTestTask("TestMalformedPayload - task 1")
	payload := TaskPayload{
		Command:      helloGoodbye(),
		MaxRunTime:   60,
		InvalidField: 119,
	}
	taskID, q := SubmitTask(t, task, payload)
	RunTestWorker(workerType, 1)
	EnsureTaskResolution(t, q, taskID, "exception", "malformed-payload")
	logtext := LogText(t, q, taskID)
	if len(logtext) == 0 {
		t.Fatal("log empty")
	}
}

// Make sure a task can run that has a command with a single token (i.e. just
// program name, no program arguments)
func TestNoProgramArgs(t *testing.T) {
}
