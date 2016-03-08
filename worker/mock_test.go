package worker

import (
	"github.com/stretchr/testify/mock"
	"github.com/taskcluster/taskcluster-client-go/queue"
	"github.com/taskcluster/taskcluster-client-go/tcclient"
)

type MockQueue struct {
	mock.Mock
}

func (m *MockQueue) ReportCompleted(taskId, runId string) (*queue.TaskStatusResponse, *tcclient.CallSummary, error) {
	args := m.Called(taskId, runId)
	return args.Get(0).(*queue.TaskStatusResponse), args.Get(1).(*tcclient.CallSummary), args.Error(2)
}

func (m *MockQueue) ReclaimTask(taskId, runId string) (*queue.TaskReclaimResponse, *tcclient.CallSummary, error) {
	args := m.Called(taskId, runId)
	return args.Get(0).(*queue.TaskReclaimResponse), args.Get(1).(*tcclient.CallSummary), args.Error(2)
}

func (m *MockQueue) PollTaskUrls(provisionerId, workerType string) (*queue.PollTaskUrlsResponse, *tcclient.CallSummary, error) {
	args := m.Called(provisionerId, workerType)
	return args.Get(0).(*queue.PollTaskUrlsResponse), args.Get(1).(*tcclient.CallSummary), args.Error(2)
}
func (m *MockQueue) CancelTask(taskId string) (*queue.TaskStatusResponse, *tcclient.CallSummary, error) {
	args := m.Called(taskId)
	return args.Get(0).(*queue.TaskStatusResponse), args.Get(1).(*tcclient.CallSummary), args.Error(2)
}
func (m *MockQueue) ClaimTask(taskId, runId string, payload *queue.TaskClaimRequest) (*queue.TaskClaimResponse, *tcclient.CallSummary, error) {
	args := m.Called(taskId, runId, payload)
	return args.Get(0).(*queue.TaskClaimResponse), args.Get(1).(*tcclient.CallSummary), args.Error(2)
}
func (m *MockQueue) ReportFailed(taskId, runId string) (*queue.TaskStatusResponse, *tcclient.CallSummary, error) {
	args := m.Called(taskId, runId)
	return args.Get(0).(*queue.TaskStatusResponse), args.Get(1).(*tcclient.CallSummary), args.Error(2)
}
func (m *MockQueue) ReportException(taskId, runId string, payload *queue.TaskExceptionRequest) (*queue.TaskStatusResponse, *tcclient.CallSummary, error) {
	args := m.Called(taskId, runId, payload)
	return args.Get(0).(*queue.TaskStatusResponse), args.Get(1).(*tcclient.CallSummary), args.Error(2)
}
