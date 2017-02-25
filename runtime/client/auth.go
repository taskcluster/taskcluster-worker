package client

import (
	"github.com/stretchr/testify/mock"
	"github.com/taskcluster/taskcluster-client-go/auth"
)

// Auth interface covers parts of the auth.Auth client that we use. This allows
// us to mock the implementation during tests.
type Auth interface {
	SentryDSN(project string) (*auth.SentryDSNResponse, error)
	StatsumToken(project string) (*auth.StatsumTokenResponse, error)
}

// MockAuth is a mock implementation of Auth for testing.
type MockAuth struct {
	mock.Mock
}

// SentryDSN is a mock implementation of SentryDSN that calls into m.Mock
func (m *MockAuth) SentryDSN(project string) (*auth.SentryDSNResponse, error) {
	args := m.Called(project)
	return args.Get(0).(*auth.SentryDSNResponse), args.Error(1)
}

// StatsumToken is a mock implementation of StatsumToken that calls into Mock
func (m *MockAuth) StatsumToken(project string) (*auth.StatsumTokenResponse, error) {
	args := m.Called(project)
	return args.Get(0).(*auth.StatsumTokenResponse), args.Error(1)
}
