package client

import "github.com/stretchr/testify/mock"

import "github.com/taskcluster/taskcluster-client-go/queue"
import "github.com/taskcluster/taskcluster-client-go/tcclient"
import "net/url"
import "time"

type MockQueue struct {
	mock.Mock
}

// CancelTask provides a mock function with given fields: _a0
func (_m *MockQueue) CancelTask(_a0 string) (*queue.TaskStatusResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0)

	var r0 *queue.TaskStatusResponse
	if rf, ok := ret.Get(0).(func(string) *queue.TaskStatusResponse); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*queue.TaskStatusResponse)
		}
	}

	var r1 *tcclient.CallSummary
	if rf, ok := ret.Get(1).(func(string) *tcclient.CallSummary); ok {
		r1 = rf(_a0)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*tcclient.CallSummary)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(string) error); ok {
		r2 = rf(_a0)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// Cert provides a mock function with given fields:
func (_m *MockQueue) Cert() (*tcclient.Certificate, error) {
	ret := _m.Called()

	var r0 *tcclient.Certificate
	if rf, ok := ret.Get(0).(func() *tcclient.Certificate); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*tcclient.Certificate)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ClaimTask provides a mock function with given fields: _a0, _a1, _a2
func (_m *MockQueue) ClaimTask(_a0 string, _a1 string, _a2 *queue.TaskClaimRequest) (*queue.TaskClaimResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 *queue.TaskClaimResponse
	if rf, ok := ret.Get(0).(func(string, string, *queue.TaskClaimRequest) *queue.TaskClaimResponse); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*queue.TaskClaimResponse)
		}
	}

	var r1 *tcclient.CallSummary
	if rf, ok := ret.Get(1).(func(string, string, *queue.TaskClaimRequest) *tcclient.CallSummary); ok {
		r1 = rf(_a0, _a1, _a2)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*tcclient.CallSummary)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(string, string, *queue.TaskClaimRequest) error); ok {
		r2 = rf(_a0, _a1, _a2)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// CreateArtifact provides a mock function with given fields: _a0, _a1, _a2, _a3
func (_m *MockQueue) CreateArtifact(_a0 string, _a1 string, _a2 string, _a3 *queue.PostArtifactRequest) (*queue.PostArtifactResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0, _a1, _a2, _a3)

	var r0 *queue.PostArtifactResponse
	if rf, ok := ret.Get(0).(func(string, string, string, *queue.PostArtifactRequest) *queue.PostArtifactResponse); ok {
		r0 = rf(_a0, _a1, _a2, _a3)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*queue.PostArtifactResponse)
		}
	}

	var r1 *tcclient.CallSummary
	if rf, ok := ret.Get(1).(func(string, string, string, *queue.PostArtifactRequest) *tcclient.CallSummary); ok {
		r1 = rf(_a0, _a1, _a2, _a3)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*tcclient.CallSummary)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(string, string, string, *queue.PostArtifactRequest) error); ok {
		r2 = rf(_a0, _a1, _a2, _a3)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// CreateNamedTemporaryCredentials provides a mock function with given fields: _a0, _a1, _a2
func (_m *MockQueue) CreateNamedTemporaryCredentials(_a0 string, _a1 time.Duration, _a2 []string) (*tcclient.Credentials, error) {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 *tcclient.Credentials
	if rf, ok := ret.Get(0).(func(string, time.Duration, []string) *tcclient.Credentials); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*tcclient.Credentials)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, time.Duration, []string) error); ok {
		r1 = rf(_a0, _a1, _a2)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// CreateTask provides a mock function with given fields: _a0, _a1
func (_m *MockQueue) CreateTask(_a0 string, _a1 *queue.TaskDefinitionRequest) (*queue.TaskStatusResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *queue.TaskStatusResponse
	if rf, ok := ret.Get(0).(func(string, *queue.TaskDefinitionRequest) *queue.TaskStatusResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*queue.TaskStatusResponse)
		}
	}

	var r1 *tcclient.CallSummary
	if rf, ok := ret.Get(1).(func(string, *queue.TaskDefinitionRequest) *tcclient.CallSummary); ok {
		r1 = rf(_a0, _a1)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*tcclient.CallSummary)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(string, *queue.TaskDefinitionRequest) error); ok {
		r2 = rf(_a0, _a1)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// CreateTemporaryCredentials provides a mock function with given fields: _a0, _a1
func (_m *MockQueue) CreateTemporaryCredentials(_a0 time.Duration, _a1 []string) (*tcclient.Credentials, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *tcclient.Credentials
	if rf, ok := ret.Get(0).(func(time.Duration, []string) *tcclient.Credentials); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*tcclient.Credentials)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(time.Duration, []string) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// DefineTask provides a mock function with given fields: _a0, _a1
func (_m *MockQueue) DefineTask(_a0 string, _a1 *queue.TaskDefinitionRequest) (*queue.TaskStatusResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *queue.TaskStatusResponse
	if rf, ok := ret.Get(0).(func(string, *queue.TaskDefinitionRequest) *queue.TaskStatusResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*queue.TaskStatusResponse)
		}
	}

	var r1 *tcclient.CallSummary
	if rf, ok := ret.Get(1).(func(string, *queue.TaskDefinitionRequest) *tcclient.CallSummary); ok {
		r1 = rf(_a0, _a1)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*tcclient.CallSummary)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(string, *queue.TaskDefinitionRequest) error); ok {
		r2 = rf(_a0, _a1)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// GetArtifact provides a mock function with given fields: _a0, _a1, _a2
func (_m *MockQueue) GetArtifact(_a0 string, _a1 string, _a2 string) (*tcclient.CallSummary, error) {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 *tcclient.CallSummary
	if rf, ok := ret.Get(0).(func(string, string, string) *tcclient.CallSummary); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*tcclient.CallSummary)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, string) error); ok {
		r1 = rf(_a0, _a1, _a2)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetArtifact_SignedURL provides a mock function with given fields: _a0, _a1, _a2, _a3
func (_m *MockQueue) GetArtifact_SignedURL(_a0 string, _a1 string, _a2 string, _a3 time.Duration) (*url.URL, error) {
	ret := _m.Called(_a0, _a1, _a2, _a3)

	var r0 *url.URL
	if rf, ok := ret.Get(0).(func(string, string, string, time.Duration) *url.URL); ok {
		r0 = rf(_a0, _a1, _a2, _a3)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*url.URL)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, string, time.Duration) error); ok {
		r1 = rf(_a0, _a1, _a2, _a3)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetLatestArtifact provides a mock function with given fields: _a0, _a1
func (_m *MockQueue) GetLatestArtifact(_a0 string, _a1 string) (*tcclient.CallSummary, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *tcclient.CallSummary
	if rf, ok := ret.Get(0).(func(string, string) *tcclient.CallSummary); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*tcclient.CallSummary)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetLatestArtifact_SignedURL provides a mock function with given fields: _a0, _a1, _a2
func (_m *MockQueue) GetLatestArtifact_SignedURL(_a0 string, _a1 string, _a2 time.Duration) (*url.URL, error) {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 *url.URL
	if rf, ok := ret.Get(0).(func(string, string, time.Duration) *url.URL); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*url.URL)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, time.Duration) error); ok {
		r1 = rf(_a0, _a1, _a2)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListArtifacts provides a mock function with given fields: _a0, _a1
func (_m *MockQueue) ListArtifacts(_a0 string, _a1 string) (*queue.ListArtifactsResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *queue.ListArtifactsResponse
	if rf, ok := ret.Get(0).(func(string, string) *queue.ListArtifactsResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*queue.ListArtifactsResponse)
		}
	}

	var r1 *tcclient.CallSummary
	if rf, ok := ret.Get(1).(func(string, string) *tcclient.CallSummary); ok {
		r1 = rf(_a0, _a1)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*tcclient.CallSummary)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(string, string) error); ok {
		r2 = rf(_a0, _a1)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// ListLatestArtifacts provides a mock function with given fields: _a0
func (_m *MockQueue) ListLatestArtifacts(_a0 string) (*queue.ListArtifactsResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0)

	var r0 *queue.ListArtifactsResponse
	if rf, ok := ret.Get(0).(func(string) *queue.ListArtifactsResponse); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*queue.ListArtifactsResponse)
		}
	}

	var r1 *tcclient.CallSummary
	if rf, ok := ret.Get(1).(func(string) *tcclient.CallSummary); ok {
		r1 = rf(_a0)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*tcclient.CallSummary)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(string) error); ok {
		r2 = rf(_a0)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// ListTaskGroup provides a mock function with given fields: _a0, _a1, _a2
func (_m *MockQueue) ListTaskGroup(_a0 string, _a1 string, _a2 string) (*queue.ListTaskGroupResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 *queue.ListTaskGroupResponse
	if rf, ok := ret.Get(0).(func(string, string, string) *queue.ListTaskGroupResponse); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*queue.ListTaskGroupResponse)
		}
	}

	var r1 *tcclient.CallSummary
	if rf, ok := ret.Get(1).(func(string, string, string) *tcclient.CallSummary); ok {
		r1 = rf(_a0, _a1, _a2)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*tcclient.CallSummary)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(string, string, string) error); ok {
		r2 = rf(_a0, _a1, _a2)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// PendingTasks provides a mock function with given fields: _a0, _a1
func (_m *MockQueue) PendingTasks(_a0 string, _a1 string) (*queue.CountPendingTasksResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *queue.CountPendingTasksResponse
	if rf, ok := ret.Get(0).(func(string, string) *queue.CountPendingTasksResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*queue.CountPendingTasksResponse)
		}
	}

	var r1 *tcclient.CallSummary
	if rf, ok := ret.Get(1).(func(string, string) *tcclient.CallSummary); ok {
		r1 = rf(_a0, _a1)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*tcclient.CallSummary)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(string, string) error); ok {
		r2 = rf(_a0, _a1)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// Ping provides a mock function with given fields:
func (_m *MockQueue) Ping() (*tcclient.CallSummary, error) {
	ret := _m.Called()

	var r0 *tcclient.CallSummary
	if rf, ok := ret.Get(0).(func() *tcclient.CallSummary); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*tcclient.CallSummary)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// PollTaskUrls provides a mock function with given fields: _a0, _a1
func (_m *MockQueue) PollTaskUrls(_a0 string, _a1 string) (*queue.PollTaskUrlsResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *queue.PollTaskUrlsResponse
	if rf, ok := ret.Get(0).(func(string, string) *queue.PollTaskUrlsResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*queue.PollTaskUrlsResponse)
		}
	}

	var r1 *tcclient.CallSummary
	if rf, ok := ret.Get(1).(func(string, string) *tcclient.CallSummary); ok {
		r1 = rf(_a0, _a1)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*tcclient.CallSummary)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(string, string) error); ok {
		r2 = rf(_a0, _a1)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// PollTaskUrls_SignedURL provides a mock function with given fields: _a0, _a1, _a2
func (_m *MockQueue) PollTaskUrls_SignedURL(_a0 string, _a1 string, _a2 time.Duration) (*url.URL, error) {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 *url.URL
	if rf, ok := ret.Get(0).(func(string, string, time.Duration) *url.URL); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*url.URL)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, time.Duration) error); ok {
		r1 = rf(_a0, _a1, _a2)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ReclaimTask provides a mock function with given fields: _a0, _a1
func (_m *MockQueue) ReclaimTask(_a0 string, _a1 string) (*queue.TaskReclaimResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *queue.TaskReclaimResponse
	if rf, ok := ret.Get(0).(func(string, string) *queue.TaskReclaimResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*queue.TaskReclaimResponse)
		}
	}

	var r1 *tcclient.CallSummary
	if rf, ok := ret.Get(1).(func(string, string) *tcclient.CallSummary); ok {
		r1 = rf(_a0, _a1)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*tcclient.CallSummary)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(string, string) error); ok {
		r2 = rf(_a0, _a1)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// ReportCompleted provides a mock function with given fields: _a0, _a1
func (_m *MockQueue) ReportCompleted(_a0 string, _a1 string) (*queue.TaskStatusResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *queue.TaskStatusResponse
	if rf, ok := ret.Get(0).(func(string, string) *queue.TaskStatusResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*queue.TaskStatusResponse)
		}
	}

	var r1 *tcclient.CallSummary
	if rf, ok := ret.Get(1).(func(string, string) *tcclient.CallSummary); ok {
		r1 = rf(_a0, _a1)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*tcclient.CallSummary)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(string, string) error); ok {
		r2 = rf(_a0, _a1)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// ReportException provides a mock function with given fields: _a0, _a1, _a2
func (_m *MockQueue) ReportException(_a0 string, _a1 string, _a2 *queue.TaskExceptionRequest) (*queue.TaskStatusResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 *queue.TaskStatusResponse
	if rf, ok := ret.Get(0).(func(string, string, *queue.TaskExceptionRequest) *queue.TaskStatusResponse); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*queue.TaskStatusResponse)
		}
	}

	var r1 *tcclient.CallSummary
	if rf, ok := ret.Get(1).(func(string, string, *queue.TaskExceptionRequest) *tcclient.CallSummary); ok {
		r1 = rf(_a0, _a1, _a2)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*tcclient.CallSummary)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(string, string, *queue.TaskExceptionRequest) error); ok {
		r2 = rf(_a0, _a1, _a2)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// ReportFailed provides a mock function with given fields: _a0, _a1
func (_m *MockQueue) ReportFailed(_a0 string, _a1 string) (*queue.TaskStatusResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *queue.TaskStatusResponse
	if rf, ok := ret.Get(0).(func(string, string) *queue.TaskStatusResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*queue.TaskStatusResponse)
		}
	}

	var r1 *tcclient.CallSummary
	if rf, ok := ret.Get(1).(func(string, string) *tcclient.CallSummary); ok {
		r1 = rf(_a0, _a1)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*tcclient.CallSummary)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(string, string) error); ok {
		r2 = rf(_a0, _a1)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// RerunTask provides a mock function with given fields: _a0
func (_m *MockQueue) RerunTask(_a0 string) (*queue.TaskStatusResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0)

	var r0 *queue.TaskStatusResponse
	if rf, ok := ret.Get(0).(func(string) *queue.TaskStatusResponse); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*queue.TaskStatusResponse)
		}
	}

	var r1 *tcclient.CallSummary
	if rf, ok := ret.Get(1).(func(string) *tcclient.CallSummary); ok {
		r1 = rf(_a0)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*tcclient.CallSummary)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(string) error); ok {
		r2 = rf(_a0)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// ScheduleTask provides a mock function with given fields: _a0
func (_m *MockQueue) ScheduleTask(_a0 string) (*queue.TaskStatusResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0)

	var r0 *queue.TaskStatusResponse
	if rf, ok := ret.Get(0).(func(string) *queue.TaskStatusResponse); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*queue.TaskStatusResponse)
		}
	}

	var r1 *tcclient.CallSummary
	if rf, ok := ret.Get(1).(func(string) *tcclient.CallSummary); ok {
		r1 = rf(_a0)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*tcclient.CallSummary)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(string) error); ok {
		r2 = rf(_a0)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// Status provides a mock function with given fields: _a0
func (_m *MockQueue) Status(_a0 string) (*queue.TaskStatusResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0)

	var r0 *queue.TaskStatusResponse
	if rf, ok := ret.Get(0).(func(string) *queue.TaskStatusResponse); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*queue.TaskStatusResponse)
		}
	}

	var r1 *tcclient.CallSummary
	if rf, ok := ret.Get(1).(func(string) *tcclient.CallSummary); ok {
		r1 = rf(_a0)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*tcclient.CallSummary)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(string) error); ok {
		r2 = rf(_a0)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// String provides a mock function with given fields:
func (_m *MockQueue) String() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// Task provides a mock function with given fields: _a0
func (_m *MockQueue) Task(_a0 string) (*queue.TaskDefinitionResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0)

	var r0 *queue.TaskDefinitionResponse
	if rf, ok := ret.Get(0).(func(string) *queue.TaskDefinitionResponse); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*queue.TaskDefinitionResponse)
		}
	}

	var r1 *tcclient.CallSummary
	if rf, ok := ret.Get(1).(func(string) *tcclient.CallSummary); ok {
		r1 = rf(_a0)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*tcclient.CallSummary)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(string) error); ok {
		r2 = rf(_a0)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}
