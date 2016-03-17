// Created by interfacer; DO NOT EDIT

package client

import (
	"github.com/taskcluster/taskcluster-client-go/index"
	"github.com/taskcluster/taskcluster-client-go/tcclient"
	"net/url"
	"time"
)

// Index is an interface generated for "github.com/taskcluster/taskcluster-client-go/index".Index.
type Index interface {
	Cert() (*tcclient.Certificate, error)
	CreateNamedTemporaryCredentials(string, time.Duration, []string) (*tcclient.Credentials, error)
	CreateTemporaryCredentials(time.Duration, []string) (*tcclient.Credentials, error)
	FindArtifactFromTask(string, string) (*tcclient.CallSummary, error)
	FindArtifactFromTask_SignedURL(string, string, time.Duration) (*url.URL, error)
	FindTask(string) (*index.IndexedTaskResponse, *tcclient.CallSummary, error)
	InsertTask(string, *index.InsertTaskRequest) (*index.IndexedTaskResponse, *tcclient.CallSummary, error)
	ListNamespaces(string, *index.ListNamespacesRequest) (*index.ListNamespacesResponse, *tcclient.CallSummary, error)
	ListTasks(string, *index.ListTasksRequest) (*index.ListTasksResponse, *tcclient.CallSummary, error)
	Ping() (*tcclient.CallSummary, error)
	String() string
}
