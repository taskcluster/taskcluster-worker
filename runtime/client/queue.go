package client

import (
	"github.com/stretchr/testify/mock"
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

func (m *MockQueue) ReportCompleted(taskID, runID string) (*queue.TaskStatusResponse, error) {
	args := m.Called(taskID, runID)
	return args.Get(0).(*queue.TaskStatusResponse), args.Error(1)
}

func (m *MockQueue) ReclaimTask(taskID, runID string) (*queue.TaskReclaimResponse, error) {
	args := m.Called(taskID, runID)
	return args.Get(0).(*queue.TaskReclaimResponse), args.Error(1)
}

func (m *MockQueue) PollTaskUrls(provisionerID, workerType string) (*queue.PollTaskUrlsResponse, error) {
	args := m.Called(provisionerID, workerType)
	return args.Get(0).(*queue.PollTaskUrlsResponse), args.Error(1)
}
func (m *MockQueue) CancelTask(taskID string) (*queue.TaskStatusResponse, error) {
	args := m.Called(taskID)
	return args.Get(0).(*queue.TaskStatusResponse), args.Error(1)
}
func (m *MockQueue) ClaimTask(taskID, runID string, payload *queue.TaskClaimRequest) (*queue.TaskClaimResponse, error) {
	args := m.Called(taskID, runID, payload)
	return args.Get(0).(*queue.TaskClaimResponse), args.Error(1)
}
func (m *MockQueue) ReportFailed(taskID, runID string) (*queue.TaskStatusResponse, error) {
	args := m.Called(taskID, runID)
	return args.Get(0).(*queue.TaskStatusResponse), args.Error(1)
}
func (m *MockQueue) ReportException(taskID, runID string, payload *queue.TaskExceptionRequest) (*queue.TaskStatusResponse, error) {
	args := m.Called(taskID, runID, payload)
	return args.Get(0).(*queue.TaskStatusResponse), args.Error(1)
}

func (m *MockQueue) CreateArtifact(taskID, runID, name string, payload *queue.PostArtifactRequest) (*queue.PostArtifactResponse, error) {
	args := m.Called(taskID, runID, name)
	return args.Get(0).(*queue.PostArtifactResponse), args.Error(1)
}
