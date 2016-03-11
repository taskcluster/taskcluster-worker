package artifacts

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-client-go/queue"
	"github.com/taskcluster/taskcluster-client-go/tcclient"
	"github.com/taskcluster/taskcluster-worker/plugins/plugintest"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
)

type artifactTestCase struct {
	TestCase  *plugintest.Case
	Artifacts string
}

func (a artifactTestCase) Test() {
	taskID := slugid.V4()
	// TODO: Do something with this error

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	s3resp, _ := json.Marshal(queue.S3ArtifactResponse{
		// TODO: Make this point to something that is always mocked. perhaps with configurable success/fail
		PutURL: ts.URL,
	})
	resp := queue.PostArtifactResponse(s3resp)
	mockedQueue := &client.MockQueue{}
	for _, path := range strings.Split(a.Artifacts, " ") {
		mockedQueue.On(
			"CreateArtifact",
			taskID,
			"0",
			path,
		).Return(&resp, &tcclient.CallSummary{}, nil)
	}

	a.TestCase.QueueMock = mockedQueue
	a.TestCase.TaskID = taskID
	a.TestCase.Test()
}

func TestArtifactsEmpty(*testing.T) {
	artifactTestCase{
		TestCase: &plugintest.Case{
			Payload: `{
				"start": {
					"delay": 0,
					"function": "true",
					"argument": "whatever"
				},
				"artifacts": []
			}`,
			Plugin:        "artifacts",
			PluginSuccess: true,
			EngineSuccess: true,
		},
	}.Test()
}

func TestArtifactsFile(*testing.T) {
	artifactTestCase{
		Artifacts: "/public /public/blah.txt",
		TestCase: &plugintest.Case{
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
						"name": "/public/blah.txt"
					}
				]
			}`,
			Plugin:        "artifacts",
			PluginSuccess: true,
			EngineSuccess: true,
		},
	}.Test()
}

func TestArtifactsDirectory(*testing.T) {
	artifactTestCase{
		Artifacts: "/public /public/blah.txt /public/foo.txt /public/bar.json",
		TestCase: &plugintest.Case{
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
						"name": "/public"
					}
				]
			}`,
			Plugin:        "artifacts",
			PluginSuccess: true,
			EngineSuccess: true,
		},
	}.Test()
}
