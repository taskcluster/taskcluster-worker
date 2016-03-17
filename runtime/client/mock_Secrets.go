package client

import "github.com/stretchr/testify/mock"

import "github.com/taskcluster/taskcluster-client-go/secrets"
import "github.com/taskcluster/taskcluster-client-go/tcclient"
import "net/url"
import "time"

type MockSecrets struct {
	mock.Mock
}

// Cert provides a mock function with given fields:
func (_m *MockSecrets) Cert() (*tcclient.Certificate, error) {
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
func (_m *MockSecrets) CreateNamedTemporaryCredentials(_a0 string, _a1 time.Duration, _a2 []string) (*tcclient.Credentials, error) {
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
func (_m *MockSecrets) CreateTemporaryCredentials(_a0 time.Duration, _a1 []string) (*tcclient.Credentials, error) {
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

// Get provides a mock function with given fields: _a0
func (_m *MockSecrets) Get(_a0 string) (*secrets.Secret, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0)

	var r0 *secrets.Secret
	if rf, ok := ret.Get(0).(func(string) *secrets.Secret); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*secrets.Secret)
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

// Get_SignedURL provides a mock function with given fields: _a0, _a1
func (_m *MockSecrets) Get_SignedURL(_a0 string, _a1 time.Duration) (*url.URL, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *url.URL
	if rf, ok := ret.Get(0).(func(string, time.Duration) *url.URL); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*url.URL)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, time.Duration) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// List provides a mock function with given fields:
func (_m *MockSecrets) List() (*secrets.SecretsList, *tcclient.CallSummary, error) {
	ret := _m.Called()

	var r0 *secrets.SecretsList
	if rf, ok := ret.Get(0).(func() *secrets.SecretsList); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*secrets.SecretsList)
		}
	}

	var r1 *tcclient.CallSummary
	if rf, ok := ret.Get(1).(func() *tcclient.CallSummary); ok {
		r1 = rf()
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*tcclient.CallSummary)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func() error); ok {
		r2 = rf()
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// Ping provides a mock function with given fields:
func (_m *MockSecrets) Ping() (*tcclient.CallSummary, error) {
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

// Remove provides a mock function with given fields: _a0
func (_m *MockSecrets) Remove(_a0 string) (*tcclient.CallSummary, error) {
	ret := _m.Called(_a0)

	var r0 *tcclient.CallSummary
	if rf, ok := ret.Get(0).(func(string) *tcclient.CallSummary); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*tcclient.CallSummary)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Set provides a mock function with given fields: _a0, _a1
func (_m *MockSecrets) Set(_a0 string, _a1 *secrets.Secret) (*tcclient.CallSummary, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *tcclient.CallSummary
	if rf, ok := ret.Get(0).(func(string, *secrets.Secret) *tcclient.CallSummary); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*tcclient.CallSummary)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, *secrets.Secret) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// String provides a mock function with given fields:
func (_m *MockSecrets) String() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}
