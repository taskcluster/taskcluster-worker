package runtime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-client-go/queue"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

func setupArtifactTest(name string, artifactResp queue.PostArtifactRequest) (*TaskContext, *client.MockQueue) {
	resp := queue.PostArtifactResponse(artifactResp)
	taskID := slugid.V4()
	context := &TaskContext{
		TaskInfo: TaskInfo{
			TaskID: taskID,
			RunID:  0,
		},
	}
	controller := &TaskContextController{context}
	mockedQueue := &client.MockQueue{}
	mockedQueue.On(
		"CreateArtifact",
		taskID,
		"0",
		name,
	).Return(&resp, nil)
	controller.SetQueueClient(mockedQueue)
	return context, mockedQueue
}

func TestS3Artifact(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	s3resp, _ := json.Marshal(queue.S3ArtifactResponse{
		PutURL: ts.URL,
	})

	artifact := &S3Artifact{
		Name:     "public/test.txt",
		Mimetype: "text/plain",
		Stream:   ioext.NopCloser(&bytes.Reader{}),
	}

	context, mockedQueue := setupArtifactTest(artifact.Name, s3resp)

	UploadS3Artifact(*artifact, context)
	mockedQueue.AssertExpectations(t)
}

func TestErrorArtifact(t *testing.T) {
	errorResp, _ := json.Marshal(queue.ErrorArtifactResponse{
		StorageType: "error",
	})

	artifact := &ErrorArtifact{
		Name:    "public/test.txt",
		Message: "Who Knows?",
		Reason:  "invalid-resource-on-worker",
	}

	context, mockedQueue := setupArtifactTest(artifact.Name, errorResp)

	CreateErrorArtifact(*artifact, context)
	mockedQueue.AssertExpectations(t)
}

func TestRedirectArtifact(t *testing.T) {
	redirResp, _ := json.Marshal(queue.ErrorArtifactResponse{
		StorageType: "reference",
	})

	artifact := &RedirectArtifact{
		Name:     "public/test.txt",
		URL:      "something.ontheweb.com",
		Mimetype: "text/plain",
	}

	context, mockedQueue := setupArtifactTest(artifact.Name, redirResp)

	CreateRedirectArtifact(*artifact, context)
	mockedQueue.AssertExpectations(t)
}

func TestPutArtifact200(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	err := putArtifact(ts.URL, "text/plain", ioext.NopCloser(&bytes.Reader{}))
	if err != nil {
		t.Error(err)
	}
}

func TestPutArtifact400(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer ts.Close()

	err := putArtifact(ts.URL, "text/plain", ioext.NopCloser(&bytes.Reader{}))
	if err == nil {
		t.Fail()
	}
}

func TestPutArtifact500(t *testing.T) {
	tries := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if tries < 3 {
			w.WriteHeader(500)
			tries++
		} else {
			w.WriteHeader(200)
		}
	}))
	defer ts.Close()

	err := putArtifact(ts.URL, "text/plain", ioext.NopCloser(&bytes.Reader{}))
	if err != nil {
		t.Error(err)
	}
}
