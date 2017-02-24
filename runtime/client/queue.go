package client

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/stretchr/testify/mock"
	tcclient "github.com/taskcluster/taskcluster-client-go"
	"github.com/taskcluster/taskcluster-client-go/queue"
)

// Queue is an interface to the Queue client provided by the
// taskcluster-client-go package.  Passing around an interface allows the
// queue client to be mocked
type Queue interface {
	ReportCompleted(string, string) (*queue.TaskStatusResponse, error)
	ReportException(string, string, *queue.TaskExceptionRequest) (*queue.TaskStatusResponse, error)
	ReportFailed(string, string) (*queue.TaskStatusResponse, error)
	ClaimTask(string, string, *queue.TaskClaimRequest) (*queue.TaskClaimResponse, error)
	ReclaimTask(string, string) (*queue.TaskReclaimResponse, error)
	PollTaskUrls(string, string) (*queue.PollTaskUrlsResponse, error)
	CancelTask(string) (*queue.TaskStatusResponse, error)
	CreateArtifact(string, string, string, *queue.PostArtifactRequest) (*queue.PostArtifactResponse, error)
}

// MockQueue is a mocked TaskCluster queue client.  Calls to methods exposed by the queue
// client will be recorded for assertion later and will respond with the data
// that was specified during creation of the mocked object.
//
// For more information about each of these client methods, consult the
// taskcluster-clieng-go/queue package
type MockQueue struct {
	mock.Mock
}

// ReportCompleted is a mock implementation of github.com/taskcluster/taskcluster-client-go/queue.ReportCompleted
func (m *MockQueue) ReportCompleted(taskID, runID string) (*queue.TaskStatusResponse, error) {
	args := m.Called(taskID, runID)
	return args.Get(0).(*queue.TaskStatusResponse), args.Error(1)
}

// ReclaimTask is a mock implementation of github.com/taskcluster/taskcluster-client-go/queue.ReclaimTask
func (m *MockQueue) ReclaimTask(taskID, runID string) (*queue.TaskReclaimResponse, error) {
	args := m.Called(taskID, runID)
	return args.Get(0).(*queue.TaskReclaimResponse), args.Error(1)
}

// PollTaskUrls is a mock implementation of github.com/taskcluster/taskcluster-client-go/queue.PollTaskUrls
func (m *MockQueue) PollTaskUrls(provisionerID, workerType string) (*queue.PollTaskUrlsResponse, error) {
	args := m.Called(provisionerID, workerType)
	return args.Get(0).(*queue.PollTaskUrlsResponse), args.Error(1)
}

// CancelTask is a mock implementation of github.com/taskcluster/taskcluster-client-go/queue.CancelTask
func (m *MockQueue) CancelTask(taskID string) (*queue.TaskStatusResponse, error) {
	args := m.Called(taskID)
	return args.Get(0).(*queue.TaskStatusResponse), args.Error(1)
}

// ClaimTask is a mock implementation of github.com/taskcluster/taskcluster-client-go/queue.ClaimTask
func (m *MockQueue) ClaimTask(taskID, runID string, payload *queue.TaskClaimRequest) (*queue.TaskClaimResponse, error) {
	args := m.Called(taskID, runID, payload)
	return args.Get(0).(*queue.TaskClaimResponse), args.Error(1)
}

// ReportFailed is a mock implementation of github.com/taskcluster/taskcluster-client-go/queue.ReportFailed
func (m *MockQueue) ReportFailed(taskID, runID string) (*queue.TaskStatusResponse, error) {
	args := m.Called(taskID, runID)
	return args.Get(0).(*queue.TaskStatusResponse), args.Error(1)
}

// ReportException is a mock implementation of github.com/taskcluster/taskcluster-client-go/queue.ReportException
func (m *MockQueue) ReportException(taskID, runID string, payload *queue.TaskExceptionRequest) (*queue.TaskStatusResponse, error) {
	args := m.Called(taskID, runID, payload)
	return args.Get(0).(*queue.TaskStatusResponse), args.Error(1)
}

// CreateArtifact is a mock implementation of github.com/taskcluster/taskcluster-client-go/queue.CreateArtifact
func (m *MockQueue) CreateArtifact(taskID, runID, name string, payload *queue.PostArtifactRequest) (*queue.PostArtifactResponse, error) {
	args := m.Called(taskID, runID, name, payload)
	return args.Get(0).(*queue.PostArtifactResponse), args.Error(1)
}

// PostAnyArtifactRequest matches if queue.PostArtifactRequest is called
var PostAnyArtifactRequest = mock.MatchedBy(func(i interface{}) bool {
	_, ok := i.(*queue.PostArtifactRequest)
	return ok
})

// PostAnyArtifactRequest matches if queue.PostArtifactRequest is called with
// an s3 artifact
var PostS3ArtifactRequest = mock.MatchedBy(func(i interface{}) bool {
	r, ok := i.(*queue.PostArtifactRequest)
	if !ok {
		return false
	}
	var s3req queue.S3ArtifactRequest
	if json.Unmarshal(*r, &s3req) != nil {
		return false
	}
	return s3req.StorageType == "s3"
})

// ExpectS3Artifact will setup queue to expect an S3 artifact with given
// name to be created for taskID and runID using m and returns
// a channel which will receive the artifact.
func (m *MockQueue) ExpectS3Artifact(taskID string, runID int, name string) <-chan []byte {
	// make channel size 100 so we don't have to handle synchronously
	c := make(chan []byte, 100)
	var s *httptest.Server
	s = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d, err := ioutil.ReadAll(r.Body)
		if err != nil {
			close(c)
			w.WriteHeader(500)
			return
		}
		if r.Header.Get("Content-Encoding") == "gzip" {
			reader, err := gzip.NewReader(bytes.NewReader(d))
			if err != nil {
				close(c)
				w.WriteHeader(500)
				return
			}
			d, err = ioutil.ReadAll(reader)
			if err != nil {
				close(c)
				w.WriteHeader(500)
				return
			}
		}
		w.WriteHeader(200)
		c <- d
		go s.Close() // Close when all requests are done (don't block the request)
	}))
	data, _ := json.Marshal(queue.S3ArtifactResponse{
		StorageType: "s3",
		PutURL:      s.URL,
		ContentType: "application/octet",
		Expires:     tcclient.Time(time.Now().Add(30 * time.Minute)),
	})
	result := queue.PostArtifactResponse(data)
	m.On(
		"CreateArtifact",
		taskID, fmt.Sprintf("%d", runID),
		name, PostS3ArtifactRequest,
	).Return(&result, nil)
	return c
}

// ExpectRedirectArtifact will setup m to expect a redirect artifact with given
// name for taskID and runID to be created. This function returns a channel for
// the url of the redirect artifact.
func (m *MockQueue) ExpectRedirectArtifact(taskID string, runID int, name string) <-chan string {
	// make channel size 100 so we don't have to handle synchronously
	c := make(chan string, 100)
	data, _ := json.Marshal(queue.RedirectArtifactResponse{
		StorageType: "reference",
	})
	result := queue.PostArtifactResponse(data)
	m.On(
		"CreateArtifact",
		taskID, fmt.Sprintf("%d", runID),
		name, PostAnyArtifactRequest,
	).Run(func(args mock.Arguments) {
		d := args.Get(3).(*queue.PostArtifactRequest)
		var r queue.RedirectArtifactRequest
		if json.Unmarshal(*d, &r) != nil {
			close(c)
			return
		}
		c <- r.URL
	}).Return(&result, nil)

	return c
}
