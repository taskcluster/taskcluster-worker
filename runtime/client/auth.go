package client

import (
	"github.com/stretchr/testify/mock"
	"github.com/taskcluster/taskcluster-client-go/tcauth"
)

// Auth interface covers parts of the tcauth.Auth client that we use. This allows
// us to mock the implementation during tests.
type Auth interface {
	SentryDSN(project string) (*tcauth.SentryDSNResponse, error)
	StatsumToken(project string) (*tcauth.StatsumTokenResponse, error)
}

// MockAuth is a mock implementation of Auth for testing.
type MockAuth struct {
	mock.Mock
}

// SentryDSN is a mock implementation of SentryDSN that calls into m.Mock
func (m *MockAuth) SentryDSN(project string) (*tcauth.SentryDSNResponse, error) {
	args := m.Called(project)
	return args.Get(0).(*tcauth.SentryDSNResponse), args.Error(1)
}

// StatsumToken is a mock implementation of StatsumToken that calls into Mock
func (m *MockAuth) StatsumToken(project string) (*tcauth.StatsumTokenResponse, error) {
	args := m.Called(project)
	return args.Get(0).(*tcauth.StatsumTokenResponse), args.Error(1)
}
