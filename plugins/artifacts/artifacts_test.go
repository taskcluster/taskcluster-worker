package artifacts

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-client-go/queue"
	"github.com/taskcluster/taskcluster-worker/plugins/plugintest"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
)

type artifactTestCase struct {
	plugintest.Case
	Artifacts []string
}

func (a artifactTestCase) Test() {
	taskID := slugid.V4()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, client.")
	}))
	defer ts.Close()

	s3resp, _ := json.Marshal(queue.S3ArtifactResponse{
		PutURL: ts.URL,
	})
	resp := queue.PostArtifactResponse(s3resp)
	mockedQueue := &client.MockQueue{}
	for _, path := range a.Artifacts {
		mockedQueue.On(
			"CreateArtifact",
			taskID,
			"0",
			path,
		).Return(&resp, nil)
	}

	a.Case.QueueMock = mockedQueue
	a.Case.TaskID = taskID
	a.Case.Test()
	mockedQueue.AssertExpectations(a.Case.TestStruct)
}

func TestArtifactsNone(t *testing.T) {
	artifactTestCase{
		Case: plugintest.Case{
			Payload: `{
				"start": {
					"delay": 0,
					"function": "true",
					"argument": "whatever"
				}
			}`,
			Plugin:        "artifacts",
			TestStruct:    t,
			PluginSuccess: true,
			EngineSuccess: true,
		},
	}.Test()
}

func TestArtifactsEmpty(t *testing.T) {
	artifactTestCase{
		Case: plugintest.Case{
			Payload: `{
				"start": {
					"delay": 0,
					"function": "true",
					"argument": "whatever"
				},
				"artifacts": []
			}`,
			Plugin:        "artifacts",
			TestStruct:    t,
			PluginSuccess: true,
			EngineSuccess: true,
		},
	}.Test()
}

func TestArtifactsFile(t *testing.T) {
	artifactTestCase{
		Artifacts: []string{"public/blah.txt"},
		Case: plugintest.Case{
			Payload: `{
				"start": {
					"delay": 0,
					"function": "write-files",
					"argument": "/artifacts/blah.txt"
				},
				"artifacts": [
					{
						"type": "file",
						"path": "/artifacts/blah.txt",
						"name": "public/blah.txt"
					}
				]
			}`,
			Plugin:        "artifacts",
			TestStruct:    t,
			PluginSuccess: true,
			EngineSuccess: true,
		},
	}.Test()
}

func TestArtifactsDirectory(t *testing.T) {
	artifactTestCase{
		Artifacts: []string{"public/blah.txt", "public/foo.txt", "public/bar.json"},
		Case: plugintest.Case{
			Payload: `{
				"start": {
					"delay": 0,
					"function": "write-files",
					"argument": "/artifacts/blah.txt /artifacts/foo.txt /artifacts/bar.json"
				},
				"artifacts": [
					{
						"type": "directory",
						"path": "/artifacts",
						"name": "public"
					}
				]
			}`,
			Plugin:        "artifacts",
			TestStruct:    t,
			PluginSuccess: true,
			EngineSuccess: true,
		},
	}.Test()
}
