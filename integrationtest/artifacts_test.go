package integrationtest

import (
	"testing"

	"github.com/taskcluster/slugid-go/slugid"
	tcclient "github.com/taskcluster/taskcluster-client-go"
	_ "github.com/taskcluster/taskcluster-worker/plugins/artifacts"
)

var (
	// all tests can share taskGroupId so we can view all test tasks in same
	// graph later for troubleshooting
	taskGroupID string = slugid.Nice()
)

func TestUpload(t *testing.T) {

	task, workerType := NewTestTask("TestUpload")
	payload := TaskPayload{
		Command: []string{
			"echo",
			"hello world",
		},
		Artifacts: []struct {
			Expires tcclient.Time `json:"expires,omitempty"`
			Name    string        `json:"name"`
			Path    string        `json:"path"`
			Type    string        `json:"type"`
		}{
			{
				Expires: task.Expires,
				Name:    "SampleArtifacts/_/X.txt",
				Path:    "SampleArtifacts/_/X.txt",
				Type:    "file",
			},
		},
	}
	taskID, _ := SubmitTask(t, task, payload)
	RunTestWorker(workerType)
	t.Log(taskID)
}
