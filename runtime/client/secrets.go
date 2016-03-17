// Created by interfacer; DO NOT EDIT

package client

import (
	"github.com/taskcluster/taskcluster-client-go/secrets"
	"github.com/taskcluster/taskcluster-client-go/tcclient"
	"net/url"
	"time"
)

// Secrets is an interface generated for "github.com/taskcluster/taskcluster-client-go/secrets".Secrets.
type Secrets interface {
	Cert() (*tcclient.Certificate, error)
	CreateNamedTemporaryCredentials(string, time.Duration, []string) (*tcclient.Credentials, error)
	CreateTemporaryCredentials(time.Duration, []string) (*tcclient.Credentials, error)
	Get(string) (*secrets.Secret, *tcclient.CallSummary, error)
	Get_SignedURL(string, time.Duration) (*url.URL, error)
	List() (*secrets.SecretsList, *tcclient.CallSummary, error)
	Ping() (*tcclient.CallSummary, error)
	Remove(string) (*tcclient.CallSummary, error)
	Set(string, *secrets.Secret) (*tcclient.CallSummary, error)
	String() string
}
