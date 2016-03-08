package worker

import (
	"github.com/stretchr/testify/mock"
	"github.com/taskcluster/taskcluster-client-go/queue"
	"github.com/taskcluster/taskcluster-client-go/tcclient"
)

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
