// Created by interfacer; DO NOT EDIT

package client

import (
	"github.com/taskcluster/taskcluster-client-go/auth"
	"github.com/taskcluster/taskcluster-client-go/tcclient"
	"net/url"
	"time"
)

// Auth is an interface generated for "github.com/taskcluster/taskcluster-client-go/auth".Auth.
type Auth interface {
	AuthenticateHawk(*auth.HawkSignatureAuthenticationRequest) (*auth.HawkSignatureAuthenticationResponse, *tcclient.CallSummary, error)
	AwsS3Credentials(string, string, string) (*auth.AWSS3CredentialsResponse, *tcclient.CallSummary, error)
	AwsS3Credentials_SignedURL(string, string, string, time.Duration) (*url.URL, error)
	AzureTableSAS(string, string) (*auth.AzureSharedAccessSignatureResponse, *tcclient.CallSummary, error)
	AzureTableSAS_SignedURL(string, string, time.Duration) (*url.URL, error)
	Cert() (*tcclient.Certificate, error)
	Client(string) (*auth.GetClientResponse, *tcclient.CallSummary, error)
	CreateClient(string, *auth.CreateClientRequest) (*auth.CreateClientResponse, *tcclient.CallSummary, error)
	CreateNamedTemporaryCredentials(string, time.Duration, []string) (*tcclient.Credentials, error)
	CreateRole(string, *auth.CreateRoleRequest) (*auth.GetRoleResponse, *tcclient.CallSummary, error)
	CreateTemporaryCredentials(time.Duration, []string) (*tcclient.Credentials, error)
	CurrentScopes() (*auth.SetOfScopes, *tcclient.CallSummary, error)
	DeleteClient(string) (*tcclient.CallSummary, error)
	DeleteRole(string) (*tcclient.CallSummary, error)
	DisableClient(string) (*auth.GetClientResponse, *tcclient.CallSummary, error)
	EnableClient(string) (*auth.GetClientResponse, *tcclient.CallSummary, error)
	ExpandScopes(*auth.SetOfScopes) (*auth.SetOfScopes, *tcclient.CallSummary, error)
	ListClients(string) (*auth.ListClientResponse, *tcclient.CallSummary, error)
	ListRoles() (*auth.ListRolesResponse, *tcclient.CallSummary, error)
	Ping() (*tcclient.CallSummary, error)
	ResetAccessToken(string) (*auth.CreateClientResponse, *tcclient.CallSummary, error)
	Role(string) (*auth.GetRoleResponse, *tcclient.CallSummary, error)
	String() string
	TestAuthenticate(*auth.TestAuthenticateRequest) (*auth.TestAuthenticateResponse, *tcclient.CallSummary, error)
	TestAuthenticateGet() (*auth.TestAuthenticateResponse, *tcclient.CallSummary, error)
	UpdateClient(string, *auth.CreateClientRequest) (*auth.GetClientResponse, *tcclient.CallSummary, error)
	UpdateRole(string, *auth.CreateRoleRequest) (*auth.GetRoleResponse, *tcclient.CallSummary, error)
}
