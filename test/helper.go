package test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/taskcluster/slugid-go/slugid"
	tcclient "github.com/taskcluster/taskcluster-client-go"
	"github.com/taskcluster/taskcluster-client-go/queue"
	"github.com/taskcluster/taskcluster-worker/commands"
	// needed so that these components register themselves
	_ "github.com/taskcluster/taskcluster-worker/commands/work"
	_ "github.com/taskcluster/taskcluster-worker/config/env"
	_ "github.com/taskcluster/taskcluster-worker/config/hostcredentials"
	_ "github.com/taskcluster/taskcluster-worker/config/secrets"
	_ "github.com/taskcluster/taskcluster-worker/engines/native"
	_ "github.com/taskcluster/taskcluster-worker/plugins/env"
	_ "github.com/taskcluster/taskcluster-worker/plugins/maxruntime"
)

var (
	// all tests can share taskGroupId so we can view all test tasks in same
	// graph later for troubleshooting
	taskGroupID = slugid.Nice()
	testdata    = testdataDir()
)

func testdataDir() string {
	// some basic setup...
	cwd, err := os.Getwd()
	if err != nil {
		panic(fmt.Errorf("Could not get current directory during test package initialisation: %v", err))
	}
	return filepath.Join(cwd, "testdata")
}

// TaskPayload is generated from running
// `taskcluster-worker schema payload [options] <config.yml>`
// and then passing the results through
// github.com/taskcluster/jsonschema2go.
// It represents the structure expected when using the test config.yml
type TaskPayload struct {

	// Artifacts to be published
	Artifacts []PayloadArtifact `json:"artifacts,omitempty"`

	// Command to execute
	Command []string `json:"command"`

	// Optional URL for a gzipped tar-ball to be downloaded and extracted in
	// the HOME directory for running the command.
	Context string `json:"context,omitempty"`

	// Mapping from environment variables to values
	Env json.RawMessage `json:"env,omitempty"`

	// If true, reboot the machine after task is finished.
	Reboot bool `json:"reboot,omitempty"`

	// Kill the task if it exceedes the timeout value.
	MaxRunTime int `json:"maxRunTime,omitempty"`
}

type PayloadArtifact struct {
	Expires tcclient.Time `json:"expires,omitempty"`

	// This will be the leading path to directories and the full name for
	// files that are uploaded to s3. It must not begin or end with "/" and
	// must only contain printable ascii characters otherwise.
	//
	// Syntax:     ^([\x20-\x2e\x30-\x7e][\x20-\x7e]*)[\x20-\x2e\x30-\x7e]$
	Name string `json:"name"`

	// File system path of artifact
	//
	// Syntax:     ^.*[^/]$
	Path string `json:"path"`

	// Artifacts can be either an individual `file` or a `directory`
	// containing potentially multiple files with recursively included
	// subdirectories
	//
	// Possible values:
	//   * "file"
	//   * "directory"
	Type string `json:"type"`
}

type Artifact struct {

	// Mimetype for the artifact that was created.
	//
	// Max length: 255
	//
	// See http://schemas.taskcluster.net/queue/v1/list-artifacts-response.json#/properties/artifacts/items/properties/contentType
	ContentType string `json:"contentType"`

	// Date and time after which the artifact created will be automatically
	// deleted by the queue.
	//
	// See http://schemas.taskcluster.net/queue/v1/list-artifacts-response.json#/properties/artifacts/items/properties/expires
	Expires tcclient.Time `json:"expires"`

	// Name of the artifact that was created, this is useful if you want to
	// attempt to fetch the artifact.
	//
	// Max length: 1024
	//
	// See http://schemas.taskcluster.net/queue/v1/list-artifacts-response.json#/properties/artifacts/items/properties/name
	Name string `json:"name"`

	// This is the `storageType` for the request that was used to create
	// the artifact.
	//
	// Possible values:
	//   * "s3"
	//   * "azure"
	//   * "reference"
	//   * "error"
	//
	// See http://schemas.taskcluster.net/queue/v1/list-artifacts-response.json#/properties/artifacts/items/properties/storageType
	StorageType string `json:"storageType"`
}

// RunTestWorker will start up the taskcluster-worker, claim one task, and then
// return. This is useful in integration tests, such that tests can submit a
// real task to the queue, then call this function to execute the task, and
// then query the queue to get task results.
func RunTestWorker(workerType string, taskCount uint16) {
	os.Setenv("TASKCLUSTER_CAPACITY", "1")
	os.Setenv("TASKCLUSTER_WORKER_TYPE", workerType)
	os.Setenv("TASKCLUSTER_WORKER_ID", workerType)
	os.Setenv("TASKCLUSTER_MAX_TASKS", fmt.Sprintf("%v", taskCount))
	commands.Run(
		[]string{
			"work",
			filepath.Join("testdata", "worker-config.yml"),
		},
	)
}

// NewTestTask generates a task definition for the given test name. This task
// definition is typically then further refined, before being sumbitted to the
// queue via a call to SubmitTask(...). It will also generate and return a
// unique (slug) workerType for this task, and this task only. This is useful
// for being able to run multiple tasks in parallel, and being confident that
// the worker instance that was started to run this task, is the one that
// receives it.
func NewTestTask(name string) (task *queue.TaskDefinitionRequest, workerType string) {
	created := time.Now().UTC()
	// reset nanoseconds
	created = created.Add(time.Nanosecond * time.Duration(created.Nanosecond()*-1))
	// deadline in one hour' time
	deadline := created.Add(15 * time.Minute)
	// expiry in one day, in case we need test results
	expires := created.AddDate(0, 0, 1)
	workerType = "dummy-worker-" + slugid.V4()[1:6]
	task = &queue.TaskDefinitionRequest{
		Created:      tcclient.Time(created),
		Deadline:     tcclient.Time(deadline),
		Expires:      tcclient.Time(expires),
		Extra:        json.RawMessage(`{}`),
		Dependencies: []string{},
		Requires:     "all-completed",
		Metadata: struct {
			Description string `json:"description"`
			Name        string `json:"name"`
			Owner       string `json:"owner"`
			Source      string `json:"source"`
		}{
			Description: name,
			Name:        name,
			Owner:       "taskcluster@mozilla.com",
			Source:      "https://github.com/taskcluster/taskcluster-worker",
		},
		Payload:       json.RawMessage(``),
		ProvisionerID: "test-dummy-provisioner",
		Retries:       1,
		Routes:        []string{},
		SchedulerID:   "test-scheduler",
		Scopes:        []string{},
		Tags:          json.RawMessage(`{}`),
		Priority:      "normal",
		TaskGroupID:   taskGroupID,
		WorkerType:    workerType,
	}
	return
}

// SubmitTask will submit a real task to the production queue, if at least
// environment variables TASKCLUSTER_CLIENT_ID and TASKCLUSTER_ACCESS_TOKEN
// have been set in the current process (TASKCLUSTER_CERTIFICATE is also
// respected, but not required). It will return a reference to the queue and
// the taskID used, in order that the caller can query the queue for results,
// if required.
func SubmitTask(
	t *testing.T,
	td *queue.TaskDefinitionRequest,
	payload TaskPayload,
) (taskID string, q *queue.Queue) {

	taskID = slugid.Nice()

	// **************************************************************
	// IF YOU UNCOMMENT THIS LINE, TestResolveResolvedTask WILL PASS.
	// Note, this just sets TASK_ID env var in the task payload, but
	// this should not be needed since worker should set this env var
	// automatically.
	//
	// Please don't check in this file with this line uncommented,
	// because we should fix the underlying problem. This code is
	// simply included for convenience, to demonstrate that having
	// the env var set correctly fixes the problem.
	// **************************************************************
	// SetTaskIDEnvVar(t, &payload.Env, taskID)

	// check we have all the env vars we need to run this test
	if os.Getenv("TASKCLUSTER_CLIENT_ID") == "" || os.Getenv("TASKCLUSTER_ACCESS_TOKEN") == "" {
		t.Skip("Skipping test since TASKCLUSTER_CLIENT_ID and/or TASKCLUSTER_ACCESS_TOKEN env vars not set")
	}
	creds := &tcclient.Credentials{
		ClientID:    os.Getenv("TASKCLUSTER_CLIENT_ID"),
		AccessToken: os.Getenv("TASKCLUSTER_ACCESS_TOKEN"),
		Certificate: os.Getenv("TASKCLUSTER_CERTIFICATE"),
	}
	q = queue.New(creds)

	b, err := json.Marshal(&payload)
	require.NoError(t, err)

	payloadJSON := json.RawMessage{}
	err = json.Unmarshal(b, &payloadJSON)
	require.NoError(t, err)

	td.Payload = payloadJSON

	// submit task
	_, err = q.CreateTask(taskID, td)
	require.NoError(t, err, "Could not submit task")
	return
}

func EnsureTaskResolution(t *testing.T, q *queue.Queue, taskID, state, reasonResolved string) {
	tsr, err := q.Status(taskID)
	if err != nil {
		t.Fatalf("Task %v: could not retrieve task status - see https://tools.taskcluster.net/task-inspector/#%v/0", taskID, taskID)
	}
	if tsr.Status.State != state {
		t.Fatalf("Task %v: was expecting task state %q but got %q - see https://tools.taskcluster.net/task-inspector/#%v/0", taskID, state, tsr.Status.State, taskID)
	}
	lastRun := tsr.Status.Runs[len(tsr.Status.Runs)-1]
	if lastRun.State != state {
		t.Fatalf("Task %v: was expecting task run state %q but got %q - see https://tools.taskcluster.net/task-inspector/#%v/0", taskID, state, lastRun.State, taskID)
	}
	if lastRun.ReasonResolved != reasonResolved {
		t.Fatalf("Task %v: was expecting task run resolved reason %q but got %q - see https://tools.taskcluster.net/task-inspector/#%v/0", taskID, reasonResolved, lastRun.ReasonResolved, taskID)
	}
}

// SetTaskIDEnvVar updates a json raw message containing an environment block,
// adding/replacing TASK_ID environment variable to have value taskID. This is
// just a temporary workaround until worker sets the environment variable
// correctly - at that point, this function can be deleted.
func SetTaskIDEnvVar(t *testing.T, rawEnv *json.RawMessage, taskID string) {
	// extract existing env vars as bytes
	bytes, err := json.Marshal(rawEnv)
	require.NoError(t, err)
	// now convert to map[string]string
	var envVars map[string]string
	err = json.Unmarshal(bytes, &envVars)
	require.NoError(t, err)
	// make sure we don't have a nil map
	if envVars == nil {
		envVars = map[string]string{}
	}
	// now set TASK_ID environment variable
	envVars["TASK_ID"] = taskID
	// convert to bytes
	bytes, err = json.Marshal(&envVars)
	require.NoError(t, err)
	// convert to RawMessage, replacing original environment block
	err = json.Unmarshal(bytes, rawEnv)
	require.NoError(t, err)
}
