package client

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/taskcluster/httpbackoff"
	tcclient "github.com/taskcluster/taskcluster-client-go"
	"github.com/taskcluster/taskcluster-client-go/queue"
)

// Queue is an interface to the Queue client provided by the
// taskcluster-client-go package.  Passing around an interface allows the
// queue client to be mocked
type Queue interface {
	Status(string) (*queue.TaskStatusResponse, error)
	ReportCompleted(string, string) (*queue.TaskStatusResponse, error)
	ReportException(string, string, *queue.TaskExceptionRequest) (*queue.TaskStatusResponse, error)
	ReportFailed(string, string) (*queue.TaskStatusResponse, error)
	ClaimTask(string, string, *queue.TaskClaimRequest) (*queue.TaskClaimResponse, error)
	ClaimWork(provisionerID, workerType string, payload *queue.ClaimWorkRequest) (*queue.ClaimWorkResponse, error)
	ReclaimTask(string, string) (*queue.TaskReclaimResponse, error)
	PollTaskUrls(string, string) (*queue.PollTaskUrlsResponse, error)
	CancelTask(string) (*queue.TaskStatusResponse, error)
	CreateArtifact(string, string, string, *queue.PostArtifactRequest) (*queue.PostArtifactResponse, error)
	GetArtifact_SignedURL(string, string, string, time.Duration) (*url.URL, error) // nolint
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

// Status is a mock implementation of github.com/taskcluster/taskcluster-client-go/queue.Status
func (m *MockQueue) Status(taskID string) (*queue.TaskStatusResponse, error) {
	args := m.Called(taskID)
	return args.Get(0).(*queue.TaskStatusResponse), args.Error(1)
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

// ClaimWork is a mock implementation of github.com/taskcluster/taskcluster-client-go/queue#Queue.ClaimWork
func (m *MockQueue) ClaimWork(provisionerID, workerType string, payload *queue.ClaimWorkRequest) (*queue.ClaimWorkResponse, error) {
	args := m.Called(provisionerID, workerType, payload)
	return args.Get(0).(*queue.ClaimWorkResponse), args.Error(1)
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

// GetArtifact_SignedURL is a mock implementation of github.com/taskcluster/taskcluster-client-go/queue.GetArtifact_SignedURL
func (m *MockQueue) GetArtifact_SignedURL(taskID, runID, name string, duration time.Duration) (*url.URL, error) { // nolint
	args := m.Called(taskID, runID, name, duration)
	return args.Get(0).(*url.URL), args.Error(1)
}

// PostAnyArtifactRequest matches if queue.PostArtifactRequest is called
var PostAnyArtifactRequest = mock.MatchedBy(func(i interface{}) bool {
	_, ok := i.(*queue.PostArtifactRequest)
	return ok
})

// PostS3ArtifactRequest matches if queue.PostS3ArtifactRequest is called with
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
		//TODO: Close the server somewhere else, doing it from in here can cause intermittent bugs!
		go func() {
			time.Sleep(60 * time.Second)
			s.Close() // Close when all requests are done (don't block the request)
		}()
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

var (
	claimWorkURLPattern = regexp.MustCompile(`^/claim-work/([^/]+)/([^/]+)$`)
	taskRunURLPattern   = regexp.MustCompile(`^/task/([^/]+)/runs/([0-9]+)/([^/]+)(?:/(.*))?$`)
	taskStatusPattern   = regexp.MustCompile(`^/task/([^/]+)/status$`)
)

// ServeHTTP handles a queue request by calling the mock implemetation
func (m *MockQueue) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	data, _ := ioutil.ReadAll(r.Body)
	r.Body.Close()
	var result interface{}
	var err error
	if match := claimWorkURLPattern.FindStringSubmatch(r.URL.Path); match != nil {
		var payload queue.ClaimWorkRequest
		if err = json.Unmarshal(data, &payload); err == nil {
			result, err = m.ClaimWork(match[1], match[2], &payload)
		}
	} else if match := taskStatusPattern.FindStringSubmatch(r.URL.Path); match != nil {
		result, err = m.Status(match[1])
	} else if match := taskRunURLPattern.FindStringSubmatch(r.URL.Path); match != nil {
		switch match[3] {
		case "reclaim":
			result, err = m.ReclaimTask(match[1], match[2])
		case "completed":
			result, err = m.ReportCompleted(match[1], match[2])
		case "failed":
			result, err = m.ReportFailed(match[1], match[2])
		case "exception":
			var payload queue.TaskExceptionRequest
			if err = json.Unmarshal(data, &payload); err == nil {
				result, err = m.ReportException(match[1], match[2], &payload)
			}
		case "artifacts":
			var payload queue.PostArtifactRequest
			if err = json.Unmarshal(data, &payload); err == nil {
				result, err = m.CreateArtifact(match[1], match[2], match[4], &payload)
			}
		}
	} else {
		w.WriteHeader(http.StatusBadRequest)
		data, _ = json.Marshal("unknown end-point: " + r.URL.Path)
		w.Write(data)
		return
	}
	if err != nil {
		if e, ok := err.(httpbackoff.BadHttpResponseCode); ok {
			w.WriteHeader(e.HttpResponseCode)
			result = e.Message
		} else if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			result = "internal-error"
		}
	} else {
		w.WriteHeader(http.StatusOK)
	}
	data, _ = json.Marshal(result)
	w.Write(data)
}
