package client

import "github.com/stretchr/testify/mock"

import "github.com/taskcluster/taskcluster-client-go/auth"
import "github.com/taskcluster/taskcluster-client-go/tcclient"
import "net/url"
import "time"

type MockAuth struct {
	mock.Mock
}

// AuthenticateHawk provides a mock function with given fields: _a0
func (_m *MockAuth) AuthenticateHawk(_a0 *auth.HawkSignatureAuthenticationRequest) (*auth.HawkSignatureAuthenticationResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0)

	var r0 *auth.HawkSignatureAuthenticationResponse
	if rf, ok := ret.Get(0).(func(*auth.HawkSignatureAuthenticationRequest) *auth.HawkSignatureAuthenticationResponse); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*auth.HawkSignatureAuthenticationResponse)
		}
	}

	var r1 *tcclient.CallSummary
	if rf, ok := ret.Get(1).(func(*auth.HawkSignatureAuthenticationRequest) *tcclient.CallSummary); ok {
		r1 = rf(_a0)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*tcclient.CallSummary)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(*auth.HawkSignatureAuthenticationRequest) error); ok {
		r2 = rf(_a0)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// AwsS3Credentials provides a mock function with given fields: _a0, _a1, _a2
func (_m *MockAuth) AwsS3Credentials(_a0 string, _a1 string, _a2 string) (*auth.AWSS3CredentialsResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 *auth.AWSS3CredentialsResponse
	if rf, ok := ret.Get(0).(func(string, string, string) *auth.AWSS3CredentialsResponse); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*auth.AWSS3CredentialsResponse)
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

// AwsS3Credentials_SignedURL provides a mock function with given fields: _a0, _a1, _a2, _a3
func (_m *MockAuth) AwsS3Credentials_SignedURL(_a0 string, _a1 string, _a2 string, _a3 time.Duration) (*url.URL, error) {
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

// AzureTableSAS provides a mock function with given fields: _a0, _a1
func (_m *MockAuth) AzureTableSAS(_a0 string, _a1 string) (*auth.AzureSharedAccessSignatureResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *auth.AzureSharedAccessSignatureResponse
	if rf, ok := ret.Get(0).(func(string, string) *auth.AzureSharedAccessSignatureResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*auth.AzureSharedAccessSignatureResponse)
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

// AzureTableSAS_SignedURL provides a mock function with given fields: _a0, _a1, _a2
func (_m *MockAuth) AzureTableSAS_SignedURL(_a0 string, _a1 string, _a2 time.Duration) (*url.URL, error) {
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

// Cert provides a mock function with given fields:
func (_m *MockAuth) Cert() (*tcclient.Certificate, error) {
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

// Client provides a mock function with given fields: _a0
func (_m *MockAuth) Client(_a0 string) (*auth.GetClientResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0)

	var r0 *auth.GetClientResponse
	if rf, ok := ret.Get(0).(func(string) *auth.GetClientResponse); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*auth.GetClientResponse)
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

// CreateClient provides a mock function with given fields: _a0, _a1
func (_m *MockAuth) CreateClient(_a0 string, _a1 *auth.CreateClientRequest) (*auth.CreateClientResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *auth.CreateClientResponse
	if rf, ok := ret.Get(0).(func(string, *auth.CreateClientRequest) *auth.CreateClientResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*auth.CreateClientResponse)
		}
	}

	var r1 *tcclient.CallSummary
	if rf, ok := ret.Get(1).(func(string, *auth.CreateClientRequest) *tcclient.CallSummary); ok {
		r1 = rf(_a0, _a1)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*tcclient.CallSummary)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(string, *auth.CreateClientRequest) error); ok {
		r2 = rf(_a0, _a1)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// CreateNamedTemporaryCredentials provides a mock function with given fields: _a0, _a1, _a2
func (_m *MockAuth) CreateNamedTemporaryCredentials(_a0 string, _a1 time.Duration, _a2 []string) (*tcclient.Credentials, error) {
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

// CreateRole provides a mock function with given fields: _a0, _a1
func (_m *MockAuth) CreateRole(_a0 string, _a1 *auth.CreateRoleRequest) (*auth.GetRoleResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *auth.GetRoleResponse
	if rf, ok := ret.Get(0).(func(string, *auth.CreateRoleRequest) *auth.GetRoleResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*auth.GetRoleResponse)
		}
	}

	var r1 *tcclient.CallSummary
	if rf, ok := ret.Get(1).(func(string, *auth.CreateRoleRequest) *tcclient.CallSummary); ok {
		r1 = rf(_a0, _a1)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*tcclient.CallSummary)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(string, *auth.CreateRoleRequest) error); ok {
		r2 = rf(_a0, _a1)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// CreateTemporaryCredentials provides a mock function with given fields: _a0, _a1
func (_m *MockAuth) CreateTemporaryCredentials(_a0 time.Duration, _a1 []string) (*tcclient.Credentials, error) {
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

// CurrentScopes provides a mock function with given fields:
func (_m *MockAuth) CurrentScopes() (*auth.SetOfScopes, *tcclient.CallSummary, error) {
	ret := _m.Called()

	var r0 *auth.SetOfScopes
	if rf, ok := ret.Get(0).(func() *auth.SetOfScopes); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*auth.SetOfScopes)
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

// DeleteClient provides a mock function with given fields: _a0
func (_m *MockAuth) DeleteClient(_a0 string) (*tcclient.CallSummary, error) {
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

// DeleteRole provides a mock function with given fields: _a0
func (_m *MockAuth) DeleteRole(_a0 string) (*tcclient.CallSummary, error) {
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

// DisableClient provides a mock function with given fields: _a0
func (_m *MockAuth) DisableClient(_a0 string) (*auth.GetClientResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0)

	var r0 *auth.GetClientResponse
	if rf, ok := ret.Get(0).(func(string) *auth.GetClientResponse); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*auth.GetClientResponse)
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

// EnableClient provides a mock function with given fields: _a0
func (_m *MockAuth) EnableClient(_a0 string) (*auth.GetClientResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0)

	var r0 *auth.GetClientResponse
	if rf, ok := ret.Get(0).(func(string) *auth.GetClientResponse); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*auth.GetClientResponse)
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

// ExpandScopes provides a mock function with given fields: _a0
func (_m *MockAuth) ExpandScopes(_a0 *auth.SetOfScopes) (*auth.SetOfScopes, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0)

	var r0 *auth.SetOfScopes
	if rf, ok := ret.Get(0).(func(*auth.SetOfScopes) *auth.SetOfScopes); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*auth.SetOfScopes)
		}
	}

	var r1 *tcclient.CallSummary
	if rf, ok := ret.Get(1).(func(*auth.SetOfScopes) *tcclient.CallSummary); ok {
		r1 = rf(_a0)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*tcclient.CallSummary)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(*auth.SetOfScopes) error); ok {
		r2 = rf(_a0)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// ListClients provides a mock function with given fields: _a0
func (_m *MockAuth) ListClients(_a0 string) (*auth.ListClientResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0)

	var r0 *auth.ListClientResponse
	if rf, ok := ret.Get(0).(func(string) *auth.ListClientResponse); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*auth.ListClientResponse)
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

// ListRoles provides a mock function with given fields:
func (_m *MockAuth) ListRoles() (*auth.ListRolesResponse, *tcclient.CallSummary, error) {
	ret := _m.Called()

	var r0 *auth.ListRolesResponse
	if rf, ok := ret.Get(0).(func() *auth.ListRolesResponse); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*auth.ListRolesResponse)
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
func (_m *MockAuth) Ping() (*tcclient.CallSummary, error) {
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

// ResetAccessToken provides a mock function with given fields: _a0
func (_m *MockAuth) ResetAccessToken(_a0 string) (*auth.CreateClientResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0)

	var r0 *auth.CreateClientResponse
	if rf, ok := ret.Get(0).(func(string) *auth.CreateClientResponse); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*auth.CreateClientResponse)
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

// Role provides a mock function with given fields: _a0
func (_m *MockAuth) Role(_a0 string) (*auth.GetRoleResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0)

	var r0 *auth.GetRoleResponse
	if rf, ok := ret.Get(0).(func(string) *auth.GetRoleResponse); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*auth.GetRoleResponse)
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
func (_m *MockAuth) String() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// TestAuthenticate provides a mock function with given fields: _a0
func (_m *MockAuth) TestAuthenticate(_a0 *auth.TestAuthenticateRequest) (*auth.TestAuthenticateResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0)

	var r0 *auth.TestAuthenticateResponse
	if rf, ok := ret.Get(0).(func(*auth.TestAuthenticateRequest) *auth.TestAuthenticateResponse); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*auth.TestAuthenticateResponse)
		}
	}

	var r1 *tcclient.CallSummary
	if rf, ok := ret.Get(1).(func(*auth.TestAuthenticateRequest) *tcclient.CallSummary); ok {
		r1 = rf(_a0)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*tcclient.CallSummary)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(*auth.TestAuthenticateRequest) error); ok {
		r2 = rf(_a0)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// TestAuthenticateGet provides a mock function with given fields:
func (_m *MockAuth) TestAuthenticateGet() (*auth.TestAuthenticateResponse, *tcclient.CallSummary, error) {
	ret := _m.Called()

	var r0 *auth.TestAuthenticateResponse
	if rf, ok := ret.Get(0).(func() *auth.TestAuthenticateResponse); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*auth.TestAuthenticateResponse)
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

// UpdateClient provides a mock function with given fields: _a0, _a1
func (_m *MockAuth) UpdateClient(_a0 string, _a1 *auth.CreateClientRequest) (*auth.GetClientResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *auth.GetClientResponse
	if rf, ok := ret.Get(0).(func(string, *auth.CreateClientRequest) *auth.GetClientResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*auth.GetClientResponse)
		}
	}

	var r1 *tcclient.CallSummary
	if rf, ok := ret.Get(1).(func(string, *auth.CreateClientRequest) *tcclient.CallSummary); ok {
		r1 = rf(_a0, _a1)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*tcclient.CallSummary)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(string, *auth.CreateClientRequest) error); ok {
		r2 = rf(_a0, _a1)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// UpdateRole provides a mock function with given fields: _a0, _a1
func (_m *MockAuth) UpdateRole(_a0 string, _a1 *auth.CreateRoleRequest) (*auth.GetRoleResponse, *tcclient.CallSummary, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *auth.GetRoleResponse
	if rf, ok := ret.Get(0).(func(string, *auth.CreateRoleRequest) *auth.GetRoleResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*auth.GetRoleResponse)
		}
	}

	var r1 *tcclient.CallSummary
	if rf, ok := ret.Get(1).(func(string, *auth.CreateRoleRequest) *tcclient.CallSummary); ok {
		r1 = rf(_a0, _a1)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*tcclient.CallSummary)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(string, *auth.CreateRoleRequest) error); ok {
		r2 = rf(_a0, _a1)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}
