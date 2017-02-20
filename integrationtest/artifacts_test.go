package integrationtest

import (
	"testing"

	"github.com/taskcluster/slugid-go/slugid"
	_ "github.com/taskcluster/taskcluster-worker/commands/work"
)

var (
	// all tests can share taskGroupId so we can view all test tasks in same
	// graph later for troubleshooting
	taskGroupID string = slugid.Nice()
)

func TestUpload(t *testing.T) {

	payload := TaskPayload{
		Command: []string{
			"echo",
			"hello world",
		},
	}
	task, workerType := NewTestTask("TestUpload")
	taskID, _ := SubmitTask(t, task, payload)
	RunTestWorker(workerType)
	t.Log(taskID)
}
