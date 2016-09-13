package livelog

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

type liveLogTest struct {
	plugintest.Case
	Artifacts []string
}

func (a liveLogTest) Test() {
	taskID := slugid.Nice()
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

func TestLiveLogging(t *testing.T) {
	liveLogTest{
		Artifacts: []string{"public/logs/live.log", "public/logs/live_backing.log"},
		Case: plugintest.Case{
			Payload: `{
				"delay": 0,
				"function": "write-log",
				"argument": "Hello world"
			}`,
			Plugin:        "livelog",
			TestStruct:    t,
			PluginSuccess: true,
			EngineSuccess: true,
			MatchLog:      "Hello world",
		},
	}.Test()
}
