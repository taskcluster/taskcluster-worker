package integrationtest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/taskcluster/slugid-go/slugid"
	tcclient "github.com/taskcluster/taskcluster-client-go"
	"github.com/taskcluster/taskcluster-client-go/queue"
	"github.com/taskcluster/taskcluster-worker/commands"
	_ "github.com/taskcluster/taskcluster-worker/commands/work"
	_ "github.com/taskcluster/taskcluster-worker/config/env"
	_ "github.com/taskcluster/taskcluster-worker/config/secrets"
)

type TaskPayload struct {

	// Artifacts to be published
	Artifacts []struct {
		Expires tcclient.Time `json:"expires,omitempty"`

		// This will be the leading path to directories and the full name
		// for files that are uploaded to s3. It must not begin or end
		// with "/" and must only contain printable ascii characters
		// otherwise.
		//
		// Syntax:     ^([\x20-\x2e\x30-\x7e][\x20-\x7e]*)[\x20-\x2e\x30-\x7e]$
		Name string `json:"name"`

		// File system path of artifact
		//
		// Syntax:     ^.*[^/]$
		Path string `json:"path"`

		// Artifacts can be either an individual `file` or a `directory` containing potentially multiple files with recursively included subdirectories
		//
		// Possible values:
		//   * "file"
		//   * "directory"
		Type string `json:"type"`
	} `json:"artifacts,omitempty"`

	// Command to execute
	Command []string `json:"command"`

	// Optional URL for a gzipped tar-ball to downloaded and extracted in the HOME directory for running the command.
	Context string `json:"context,omitempty"`

	// Mapping from environment variables to values
	Env json.RawMessage `json:"env,omitempty"`

	// If true, reboot the machine after task is finished.
	Reboot bool `json:"reboot,omitempty"`
}

func RunTestWorker(workerType string) {
	os.Setenv("TASKCLUSTER_CAPACITY", "1")
	os.Setenv("TASKCLUSTER_WORKER_TYPE", workerType)
	os.Setenv("TASKCLUSTER_WORKER_ID", workerType)
	os.Setenv("TASKCLUSTER_MAX_TASKS", "1")
	commands.Run(
		[]string{
			"work",
			filepath.Join("testdata", "worker-config.yml"),
		},
	)
}

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

func SubmitTask(t *testing.T, td *queue.TaskDefinitionRequest, payload TaskPayload) (taskID string, myQueue *queue.Queue) {
	taskID = slugid.Nice()
	// check we have all the env vars we need to run this test
	if os.Getenv("TASKCLUSTER_CLIENT_ID") == "" || os.Getenv("TASKCLUSTER_ACCESS_TOKEN") == "" {
		t.Skip("Skipping test since TASKCLUSTER_CLIENT_ID and/or TASKCLUSTER_ACCESS_TOKEN env vars not set")
	}
	creds := &tcclient.Credentials{
		ClientID:    os.Getenv("TASKCLUSTER_CLIENT_ID"),
		AccessToken: os.Getenv("TASKCLUSTER_ACCESS_TOKEN"),
		Certificate: os.Getenv("TASKCLUSTER_CERTIFICATE"),
	}
	myQueue = queue.New(creds)

	b, err := json.Marshal(&payload)
	if err != nil {
		t.Fatalf("Could not convert task payload to json")
	}

	payloadJSON := json.RawMessage{}
	err = json.Unmarshal(b, &payloadJSON)
	if err != nil {
		t.Fatalf("Could not convert json bytes of payload to json.RawMessage")
	}

	td.Payload = payloadJSON

	// submit task
	_, err = myQueue.CreateTask(taskID, td)
	if err != nil {
		t.Fatalf("Could not submit task: %v", err)
	}
	return
}
