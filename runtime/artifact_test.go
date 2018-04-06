package runtime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-client-go/tcqueue"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

func setupArtifactTest(name string, artifactResp tcqueue.PostArtifactRequest) (*TaskContext, *client.MockQueue) {
	resp := tcqueue.PostArtifactResponse(artifactResp)
	taskID := slugid.Nice()
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
		client.PostAnyArtifactRequest,
	).Return(&resp, nil)
	controller.SetQueueClient(mockedQueue)
	return context, mockedQueue
}

func TestS3Artifact(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	s3resp, _ := json.Marshal(tcqueue.S3ArtifactResponse{
		PutURL: ts.URL,
	})

	artifact := &S3Artifact{
		Name:     "public/test.txt",
		Mimetype: "text/plain; charset=utf-8",
		Stream:   ioext.NopCloser(&bytes.Reader{}),
	}

	context, mockedQueue := setupArtifactTest(artifact.Name, s3resp)

	context.UploadS3Artifact(*artifact)
	mockedQueue.AssertExpectations(t)
}

func TestErrorArtifact(t *testing.T) {
	errorResp, _ := json.Marshal(tcqueue.ErrorArtifactResponse{
		StorageType: "error",
	})

	artifact := &ErrorArtifact{
		Name:    "public/test.txt",
		Message: "Who Knows?",
		Reason:  "invalid-resource-on-worker",
	}

	context, mockedQueue := setupArtifactTest(artifact.Name, errorResp)

	context.CreateErrorArtifact(*artifact)
	mockedQueue.AssertExpectations(t)
}

func TestRedirectArtifact(t *testing.T) {
	redirResp, _ := json.Marshal(tcqueue.ErrorArtifactResponse{
		StorageType: "reference",
	})

	artifact := &RedirectArtifact{
		Name:     "public/test.txt",
		URL:      "something.ontheweb.com",
		Mimetype: "text/plain; charset=utf-8",
	}

	context, mockedQueue := setupArtifactTest(artifact.Name, redirResp)

	context.CreateRedirectArtifact(*artifact)
	mockedQueue.AssertExpectations(t)
}

func TestPutArtifact200(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	err := putArtifact(ts.URL, "text/plain; charset=utf-8", ioext.NopCloser(&bytes.Reader{}), map[string]string{})
	if err != nil {
		t.Error(err)
	}
}

func TestPutArtifact400(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer ts.Close()

	err := putArtifact(ts.URL, "text/plain; charset=utf-8", ioext.NopCloser(&bytes.Reader{}), map[string]string{})
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

	err := putArtifact(ts.URL, "text/plain; charset=utf-8", ioext.NopCloser(&bytes.Reader{}), map[string]string{})
	if err != nil {
		t.Error(err)
	}
}
