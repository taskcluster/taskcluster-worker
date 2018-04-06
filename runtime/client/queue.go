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
	"github.com/taskcluster/taskcluster-client-go/tcqueue"
)

// Queue is an interface to the Queue client provided by the
// taskcluster-client-go package.  Passing around an interface allows the
// queue client to be mocked
type Queue interface {
	Status(string) (*tcqueue.TaskStatusResponse, error)
	ReportCompleted(string, string) (*tcqueue.TaskStatusResponse, error)
	ReportException(string, string, *tcqueue.TaskExceptionRequest) (*tcqueue.TaskStatusResponse, error)
	ReportFailed(string, string) (*tcqueue.TaskStatusResponse, error)
	ClaimTask(string, string, *tcqueue.TaskClaimRequest) (*tcqueue.TaskClaimResponse, error)
	ClaimWork(provisionerID, workerType string, payload *tcqueue.ClaimWorkRequest) (*tcqueue.ClaimWorkResponse, error)
	ReclaimTask(string, string) (*tcqueue.TaskReclaimResponse, error)
	PollTaskUrls(string, string) (*tcqueue.PollTaskUrlsResponse, error)
	CancelTask(string) (*tcqueue.TaskStatusResponse, error)
	CreateArtifact(string, string, string, *tcqueue.PostArtifactRequest) (*tcqueue.PostArtifactResponse, error)
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

// Status is a mock implementation of github.com/taskcluster/taskcluster-client-go/tcqueue.Status
func (m *MockQueue) Status(taskID string) (*tcqueue.TaskStatusResponse, error) {
	args := m.Called(taskID)
	return args.Get(0).(*tcqueue.TaskStatusResponse), args.Error(1)
}

// ReportCompleted is a mock implementation of github.com/taskcluster/taskcluster-client-go/tcqueue.ReportCompleted
func (m *MockQueue) ReportCompleted(taskID, runID string) (*tcqueue.TaskStatusResponse, error) {
	args := m.Called(taskID, runID)
	return args.Get(0).(*tcqueue.TaskStatusResponse), args.Error(1)
}

// ReclaimTask is a mock implementation of github.com/taskcluster/taskcluster-client-go/tcqueue.ReclaimTask
func (m *MockQueue) ReclaimTask(taskID, runID string) (*tcqueue.TaskReclaimResponse, error) {
	args := m.Called(taskID, runID)
	return args.Get(0).(*tcqueue.TaskReclaimResponse), args.Error(1)
}

// PollTaskUrls is a mock implementation of github.com/taskcluster/taskcluster-client-go/tcqueue.PollTaskUrls
func (m *MockQueue) PollTaskUrls(provisionerID, workerType string) (*tcqueue.PollTaskUrlsResponse, error) {
	args := m.Called(provisionerID, workerType)
	return args.Get(0).(*tcqueue.PollTaskUrlsResponse), args.Error(1)
}

// CancelTask is a mock implementation of github.com/taskcluster/taskcluster-client-go/tcqueue.CancelTask
func (m *MockQueue) CancelTask(taskID string) (*tcqueue.TaskStatusResponse, error) {
	args := m.Called(taskID)
	return args.Get(0).(*tcqueue.TaskStatusResponse), args.Error(1)
}

// ClaimTask is a mock implementation of github.com/taskcluster/taskcluster-client-go/tcqueue.ClaimTask
func (m *MockQueue) ClaimTask(taskID, runID string, payload *tcqueue.TaskClaimRequest) (*tcqueue.TaskClaimResponse, error) {
	args := m.Called(taskID, runID, payload)
	return args.Get(0).(*tcqueue.TaskClaimResponse), args.Error(1)
}

// ClaimWork is a mock implementation of github.com/taskcluster/taskcluster-client-go/queue#Queue.ClaimWork
func (m *MockQueue) ClaimWork(provisionerID, workerType string, payload *tcqueue.ClaimWorkRequest) (*tcqueue.ClaimWorkResponse, error) {
	args := m.Called(provisionerID, workerType, payload)
	return args.Get(0).(*tcqueue.ClaimWorkResponse), args.Error(1)
}

// ReportFailed is a mock implementation of github.com/taskcluster/taskcluster-client-go/tcqueue.ReportFailed
func (m *MockQueue) ReportFailed(taskID, runID string) (*tcqueue.TaskStatusResponse, error) {
	args := m.Called(taskID, runID)
	return args.Get(0).(*tcqueue.TaskStatusResponse), args.Error(1)
}

// ReportException is a mock implementation of github.com/taskcluster/taskcluster-client-go/tcqueue.ReportException
func (m *MockQueue) ReportException(taskID, runID string, payload *tcqueue.TaskExceptionRequest) (*tcqueue.TaskStatusResponse, error) {
	args := m.Called(taskID, runID, payload)
	return args.Get(0).(*tcqueue.TaskStatusResponse), args.Error(1)
}

// CreateArtifact is a mock implementation of github.com/taskcluster/taskcluster-client-go/tcqueue.CreateArtifact
func (m *MockQueue) CreateArtifact(taskID, runID, name string, payload *tcqueue.PostArtifactRequest) (*tcqueue.PostArtifactResponse, error) {
	args := m.Called(taskID, runID, name, payload)
	return args.Get(0).(*tcqueue.PostArtifactResponse), args.Error(1)
}

// GetArtifact_SignedURL is a mock implementation of github.com/taskcluster/taskcluster-client-go/tcqueue.GetArtifact_SignedURL
func (m *MockQueue) GetArtifact_SignedURL(taskID, runID, name string, duration time.Duration) (*url.URL, error) { // nolint
	args := m.Called(taskID, runID, name, duration)
	return args.Get(0).(*url.URL), args.Error(1)
}

// PostAnyArtifactRequest matches if tcqueue.PostArtifactRequest is called
var PostAnyArtifactRequest = mock.MatchedBy(func(i interface{}) bool {
	_, ok := i.(*tcqueue.PostArtifactRequest)
	return ok
})

// PostS3ArtifactRequest matches if tcqueue.PostS3ArtifactRequest is called with
// an s3 artifact
var PostS3ArtifactRequest = mock.MatchedBy(func(i interface{}) bool {
	r, ok := i.(*tcqueue.PostArtifactRequest)
	if !ok {
		return false
	}
	var s3req tcqueue.S3ArtifactRequest
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
	data, _ := json.Marshal(tcqueue.S3ArtifactResponse{
		StorageType: "s3",
		PutURL:      s.URL,
		ContentType: "application/octet",
		Expires:     tcclient.Time(time.Now().Add(30 * time.Minute)),
	})
	result := tcqueue.PostArtifactResponse(data)
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
	data, _ := json.Marshal(tcqueue.RedirectArtifactResponse{
		StorageType: "reference",
	})
	result := tcqueue.PostArtifactResponse(data)
	m.On(
		"CreateArtifact",
		taskID, fmt.Sprintf("%d", runID),
		name, PostAnyArtifactRequest,
	).Run(func(args mock.Arguments) {
		d := args.Get(3).(*tcqueue.PostArtifactRequest)
		var r tcqueue.RedirectArtifactRequest
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
		var payload tcqueue.ClaimWorkRequest
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
			var payload tcqueue.TaskExceptionRequest
			if err = json.Unmarshal(data, &payload); err == nil {
				result, err = m.ReportException(match[1], match[2], &payload)
			}
		case "artifacts":
			var payload tcqueue.PostArtifactRequest
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
