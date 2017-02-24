package client

import "github.com/taskcluster/taskcluster-client-go/auth"

// Auth interface covers parts of the auth.Auth client that we use. This allows
// us to mock the implementation during tests.
type Auth interface {
	SentryDSN(project string) (*auth.SentryDSNResponse, error)
	StatsumToken(project string) (*auth.StatsumTokenResponse, error)
}
