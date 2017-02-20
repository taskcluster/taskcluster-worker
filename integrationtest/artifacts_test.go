package integrationtest

import (
	"testing"

	"github.com/taskcluster/slugid-go/slugid"
	_ "github.com/taskcluster/taskcluster-worker/plugins/artifacts"
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
		Artifacts: []struct {
			Expires tcclient.Time `json:"expires"`
			Name    string        `json:"type"`
			Path    string        `json:"path"`
			Type    string        `json:"type"`
		}{
			{
				Expires: expires,
				Name:    "SampleArtifacts/_/X.txt",
				Path:    "SampleArtifacts/_/X.txt",
				Type:    "file",
			},
		},
	}
	task, workerType := NewTestTask("TestUpload")
	taskID, _ := SubmitTask(t, task, payload)
	RunTestWorker(workerType)
	t.Log(taskID)
}
