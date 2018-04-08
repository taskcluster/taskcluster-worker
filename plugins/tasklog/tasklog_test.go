package tasklog

import (
	"compress/gzip"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-client-go/tcqueue"
	"github.com/taskcluster/taskcluster-worker/plugins/plugintest"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
)

func TestTaskLog(t *testing.T) {
	taskID := slugid.Nice()

	// Simulated S3 server
	s3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := gzip.NewReader(r.Body)
		require.NoError(t, err, "failed to read gzipped body (simulated s3 server)")
		data, err := ioutil.ReadAll(body)
		require.NoError(t, err, "failed to read body (simulated s3 server)")

		debug("got data: '%s'", string(data))
		require.Contains(t, string(data), "magic-words-to-look-for")

		w.WriteHeader(http.StatusOK)
	}))
	defer s3.Close()

	s3resp, _ := json.Marshal(tcqueue.S3ArtifactResponse{
		PutURL: s3.URL,
	})
	resp := tcqueue.PostArtifactResponse(s3resp)
	queueMock := &client.MockQueue{}
	queueMock.On(
		"CreateArtifact",
		taskID,
		"0",
		"public/logs/task.log",
		client.PostS3ArtifactRequest,
	).Return(&resp, nil)

	// Define test case
	plugintest.Case{
		TaskID:    taskID,
		QueueMock: queueMock,
		Payload: `{
			"delay": 0,
			"function": "write-log",
			"argument": "magic-words-to-look-for"
		}`,
		Plugin:        "tasklog",
		PluginConfig:  `{}`,
		PluginSuccess: true,
		EngineSuccess: true,
	}.Test()

	// test mock assertions
	queueMock.AssertExpectations(t)
}
