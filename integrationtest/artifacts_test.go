package integrationtest

import (
	"path/filepath"
	"runtime"
	"testing"

	tcclient "github.com/taskcluster/taskcluster-client-go"
	_ "github.com/taskcluster/taskcluster-worker/plugins/artifacts"
	_ "github.com/taskcluster/taskcluster-worker/plugins/livelog"
)

func TestUpload(t *testing.T) {

	if runtime.GOOS == "windows" {
		t.Skip("Currently not supported on Windows")
	}

	task, workerType := NewTestTask("TestUpload")
	payload := TaskPayload{
		Command: []string{
			"pwd",
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
				Path:    filepath.Join(testdata, "SampleArtifacts/_/X.txt"),
				Type:    "file",
			},
		},
	}
	SubmitTask(t, task, payload)
	RunTestWorker(workerType)

	// now check
}
