package runtime

import (
	"github.com/stretchr/testify/mock"
	"github.com/taskcluster/taskcluster-client-go/queue"
	"github.com/taskcluster/taskcluster-client-go/tcclient"
)

// QueueClient is an interface to the Queue client provided by the
// taskcluster-client-go package.  Passing around an interface allows the
// queue client to be mocked
type QueueClient interface {
	ReportCompleted(string, string) (*queue.TaskStatusResponse, *tcclient.CallSummary, error)
	ReportException(string, string, *queue.TaskExceptionRequest) (*queue.TaskStatusResponse, *tcclient.CallSummary, error)
	ReportFailed(string, string) (*queue.TaskStatusResponse, *tcclient.CallSummary, error)
	ClaimTask(string, string, *queue.TaskClaimRequest) (*queue.TaskClaimResponse, *tcclient.CallSummary, error)
	ReclaimTask(string, string) (*queue.TaskReclaimResponse, *tcclient.CallSummary, error)
	PollTaskUrls(string, string) (*queue.PollTaskUrlsResponse, *tcclient.CallSummary, error)
	CancelTask(string) (*queue.TaskStatusResponse, *tcclient.CallSummary, error)
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

func (m *MockQueue) ReportCompleted(taskID, runID string) (*queue.TaskStatusResponse, *tcclient.CallSummary, error) {
	args := m.Called(taskID, runID)
	return args.Get(0).(*queue.TaskStatusResponse), args.Get(1).(*tcclient.CallSummary), args.Error(2)
}

func (m *MockQueue) ReclaimTask(taskID, runID string) (*queue.TaskReclaimResponse, *tcclient.CallSummary, error) {
	args := m.Called(taskID, runID)
	return args.Get(0).(*queue.TaskReclaimResponse), args.Get(1).(*tcclient.CallSummary), args.Error(2)
}

func (m *MockQueue) PollTaskUrls(provisionerID, workerType string) (*queue.PollTaskUrlsResponse, *tcclient.CallSummary, error) {
	args := m.Called(provisionerID, workerType)
	return args.Get(0).(*queue.PollTaskUrlsResponse), args.Get(1).(*tcclient.CallSummary), args.Error(2)
}
func (m *MockQueue) CancelTask(taskID string) (*queue.TaskStatusResponse, *tcclient.CallSummary, error) {
	args := m.Called(taskID)
	return args.Get(0).(*queue.TaskStatusResponse), args.Get(1).(*tcclient.CallSummary), args.Error(2)
}
func (m *MockQueue) ClaimTask(taskID, runID string, payload *queue.TaskClaimRequest) (*queue.TaskClaimResponse, *tcclient.CallSummary, error) {
	args := m.Called(taskID, runID, payload)
	return args.Get(0).(*queue.TaskClaimResponse), args.Get(1).(*tcclient.CallSummary), args.Error(2)
}
func (m *MockQueue) ReportFailed(taskID, runID string) (*queue.TaskStatusResponse, *tcclient.CallSummary, error) {
	args := m.Called(taskID, runID)
	return args.Get(0).(*queue.TaskStatusResponse), args.Get(1).(*tcclient.CallSummary), args.Error(2)
}
func (m *MockQueue) ReportException(taskID, runID string, payload *queue.TaskExceptionRequest) (*queue.TaskStatusResponse, *tcclient.CallSummary, error) {
	args := m.Called(taskID, runID, payload)
	return args.Get(0).(*queue.TaskStatusResponse), args.Get(1).(*tcclient.CallSummary), args.Error(2)
}
