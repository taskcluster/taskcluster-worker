package client

import "github.com/stretchr/testify/mock"

import "github.com/taskcluster/taskcluster-client-go/index"
import "github.com/taskcluster/taskcluster-client-go/tcclient"
import "net/url"
import "time"

type MockIndex struct {
	mock.Mock
}

// Cert provides a mock function with given fields:
func (_m *MockIndex) Cert() (*tcclient.Certificate, error) {
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

// CreateNamedTemporaryCredentials provides a mock function with given fields: _a0, _a1, _a2
func (_m *MockIndex) CreateNamedTemporaryCredentials(_a0 string, _a1 time.Duration, _a2 []string) (*tcclient.Credentials, error) {
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

// CreateTemporaryCredentials provides a mock function with given fields: _a0, _a1
func (_m *MockIndex) CreateTemporaryCredentials(_a0 time.Duration, _a1 []string) (*tcclient.Credentials, error) {
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

// FindArtifactFromTask provides a mock function with given fields: _a0, _a1
func (_m *MockIndex) FindArtifactFromTask(_a0 string, _a1 string) (*tcclient.CallSummary, error) {
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

// FindArtifactFromTask_SignedURL provides a mock function with given fields: _a0, _a1, _a2
func (_m *MockIndex) FindArtifactFromTask_SignedURL(_a0 string, _a1 string, _a2 time.Duration) (*url.URL, error) {
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

// FindTask provides a mock function with given fields: _a0
func (_m *MockIndex) FindTask(_a0 string) (*index.IndexedTaskResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0)

	var r0 *index.IndexedTaskResponse
	if rf, ok := ret.Get(0).(func(string) *index.IndexedTaskResponse); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*index.IndexedTaskResponse)
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

// InsertTask provides a mock function with given fields: _a0, _a1
func (_m *MockIndex) InsertTask(_a0 string, _a1 *index.InsertTaskRequest) (*index.IndexedTaskResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *index.IndexedTaskResponse
	if rf, ok := ret.Get(0).(func(string, *index.InsertTaskRequest) *index.IndexedTaskResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*index.IndexedTaskResponse)
		}
	}

	var r1 *tcclient.CallSummary
	if rf, ok := ret.Get(1).(func(string, *index.InsertTaskRequest) *tcclient.CallSummary); ok {
		r1 = rf(_a0, _a1)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*tcclient.CallSummary)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(string, *index.InsertTaskRequest) error); ok {
		r2 = rf(_a0, _a1)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// ListNamespaces provides a mock function with given fields: _a0, _a1
func (_m *MockIndex) ListNamespaces(_a0 string, _a1 *index.ListNamespacesRequest) (*index.ListNamespacesResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *index.ListNamespacesResponse
	if rf, ok := ret.Get(0).(func(string, *index.ListNamespacesRequest) *index.ListNamespacesResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*index.ListNamespacesResponse)
		}
	}

	var r1 *tcclient.CallSummary
	if rf, ok := ret.Get(1).(func(string, *index.ListNamespacesRequest) *tcclient.CallSummary); ok {
		r1 = rf(_a0, _a1)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*tcclient.CallSummary)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(string, *index.ListNamespacesRequest) error); ok {
		r2 = rf(_a0, _a1)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// ListTasks provides a mock function with given fields: _a0, _a1
func (_m *MockIndex) ListTasks(_a0 string, _a1 *index.ListTasksRequest) (*index.ListTasksResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *index.ListTasksResponse
	if rf, ok := ret.Get(0).(func(string, *index.ListTasksRequest) *index.ListTasksResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*index.ListTasksResponse)
		}
	}

	var r1 *tcclient.CallSummary
	if rf, ok := ret.Get(1).(func(string, *index.ListTasksRequest) *tcclient.CallSummary); ok {
		r1 = rf(_a0, _a1)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*tcclient.CallSummary)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(string, *index.ListTasksRequest) error); ok {
		r2 = rf(_a0, _a1)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// Ping provides a mock function with given fields:
func (_m *MockIndex) Ping() (*tcclient.CallSummary, error) {
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

// String provides a mock function with given fields:
func (_m *MockIndex) String() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}
