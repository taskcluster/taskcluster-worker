// Created by interfacer; DO NOT EDIT

package client

import (
	"github.com/taskcluster/taskcluster-client-go/queue"
	"github.com/taskcluster/taskcluster-client-go/tcclient"
	"net/url"
	"time"
)

// Queue is an interface generated for "github.com/taskcluster/taskcluster-client-go/queue".Queue.
type Queue interface {
	CancelTask(string) (*queue.TaskStatusResponse, *tcclient.CallSummary, error)
	Cert() (*tcclient.Certificate, error)
	ClaimTask(string, string, *queue.TaskClaimRequest) (*queue.TaskClaimResponse, *tcclient.CallSummary, error)
	CreateArtifact(string, string, string, *queue.PostArtifactRequest) (*queue.PostArtifactResponse, *tcclient.CallSummary, error)
	CreateNamedTemporaryCredentials(string, time.Duration, []string) (*tcclient.Credentials, error)
	CreateTask(string, *queue.TaskDefinitionRequest) (*queue.TaskStatusResponse, *tcclient.CallSummary, error)
	CreateTemporaryCredentials(time.Duration, []string) (*tcclient.Credentials, error)
	DefineTask(string, *queue.TaskDefinitionRequest) (*queue.TaskStatusResponse, *tcclient.CallSummary, error)
	GetArtifact(string, string, string) (*tcclient.CallSummary, error)
	GetArtifact_SignedURL(string, string, string, time.Duration) (*url.URL, error)
	GetLatestArtifact(string, string) (*tcclient.CallSummary, error)
	GetLatestArtifact_SignedURL(string, string, time.Duration) (*url.URL, error)
	ListArtifacts(string, string) (*queue.ListArtifactsResponse, *tcclient.CallSummary, error)
	ListLatestArtifacts(string) (*queue.ListArtifactsResponse, *tcclient.CallSummary, error)
	ListTaskGroup(string, string, string) (*queue.ListTaskGroupResponse, *tcclient.CallSummary, error)
	PendingTasks(string, string) (*queue.CountPendingTasksResponse, *tcclient.CallSummary, error)
	Ping() (*tcclient.CallSummary, error)
	PollTaskUrls(string, string) (*queue.PollTaskUrlsResponse, *tcclient.CallSummary, error)
	PollTaskUrls_SignedURL(string, string, time.Duration) (*url.URL, error)
	ReclaimTask(string, string) (*queue.TaskReclaimResponse, *tcclient.CallSummary, error)
	ReportCompleted(string, string) (*queue.TaskStatusResponse, *tcclient.CallSummary, error)
	ReportException(string, string, *queue.TaskExceptionRequest) (*queue.TaskStatusResponse, *tcclient.CallSummary, error)
	ReportFailed(string, string) (*queue.TaskStatusResponse, *tcclient.CallSummary, error)
	RerunTask(string) (*queue.TaskStatusResponse, *tcclient.CallSummary, error)
	ScheduleTask(string) (*queue.TaskStatusResponse, *tcclient.CallSummary, error)
	Status(string) (*queue.TaskStatusResponse, *tcclient.CallSummary, error)
	String() string
	Task(string) (*queue.TaskDefinitionResponse, *tcclient.CallSummary, error)
}
