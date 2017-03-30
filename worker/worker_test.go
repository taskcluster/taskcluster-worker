package worker

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/taskcluster/httpbackoff"
	"github.com/taskcluster/slugid-go/slugid"
	tcclient "github.com/taskcluster/taskcluster-client-go"
	"github.com/taskcluster/taskcluster-client-go/queue"
	_ "github.com/taskcluster/taskcluster-worker/engines/mock"
	_ "github.com/taskcluster/taskcluster-worker/plugins/success"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
)

func setupTestWorker(t *testing.T, queueBaseURL string, concurrency int) *Worker {
	raw := `{
		"engine": "mock",
		"engines": {
			"mock": {}
		},
		"plugins": {
			"disabled": [],
			"success": {}
		},
		"webHookServer": {"provider": "localhost"},
		"temporaryFolder": "` + path.Join(os.TempDir(), slugid.Nice()) + `",
		"minimumDiskSpace": 0,
		"minimumMemory": 0,
		"monitor": {"type": "mock", "panicOnError": true},
		"credentials": {
			"clientId": "my-test-client-id",
			"accessToken": "my-super-secret-access-token"
		},
		"queueBaseUrl": "` + queueBaseURL + `",
		"worker": {
			"provisionerId": "test-provisioner-id",
			"workerType": "test-worker-type",
			"workerGroup": "test-worker-group",
			"workerId": "test-worker-id",
			"pollingInterval": 1,
			"reclaimOffset": 1,
			"minimumReclaimDelay": 1,
			"concurrency": ` + strconv.Itoa(concurrency) + `
		}
	}`
	var config interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &config))
	w, err := New(config)
	require.NoError(t, err)
	return w
}

func TestWorkerClaimWork(t *testing.T) {
	// Set mock queue, server and worker
	q := client.MockQueue{}
	s := httptest.NewServer(&q)
	defer s.Close()
	defer q.AssertExpectations(t)
	w := setupTestWorker(t, s.URL, 2)
	defer w.Start()

	// Model the queue
	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", mock.Anything).Twice().Return(&queue.ClaimWorkResponse{
		Tasks: append(queue.ClaimWorkResponse{}.Tasks),
	}, nil)

	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", mock.Anything).Once().Run(func(args mock.Arguments) {
		w.StopGracefully()
	}).Return(&queue.ClaimWorkResponse{
		Tasks: append(queue.ClaimWorkResponse{}.Tasks),
	}, nil)
}

func TestWorkerProcessTasks(t *testing.T) {
	// Set mock queue, server and worker
	q := client.MockQueue{}
	s := httptest.NewServer(&q)
	defer s.Close()
	defer q.AssertExpectations(t)
	w := setupTestWorker(t, s.URL, 1)
	defer w.Start()

	// Model the queue

	// return no task, once
	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", mock.Anything).Once().Return(&queue.ClaimWorkResponse{
		Tasks: append(queue.ClaimWorkResponse{}.Tasks),
	}, nil)

	// return a task, once
	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", mock.Anything).Once().Return(&queue.ClaimWorkResponse{
		Tasks: append(queue.ClaimWorkResponse{}.Tasks, taskClaim{
			Status:     queue.TaskStatusStructure{TaskID: "my-task-id-1"},
			RunID:      0,
			TakenUntil: tcclient.Time(time.Now().Add(10 * time.Minute)),
			Task: queue.TaskDefinitionResponse{
				Payload: json.RawMessage(`{
					"delay": 200,
					"function": "true",
					"argument": ""
				}`),
			},
		}),
	}, nil)
	q.On("ReportCompleted", "my-task-id-1", "0").Once().Return(&queue.TaskStatusResponse{}, nil)

	// return a task, once
	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", mock.Anything).Once().Return(&queue.ClaimWorkResponse{
		Tasks: append(queue.ClaimWorkResponse{}.Tasks, taskClaim{
			Status:     queue.TaskStatusStructure{TaskID: "my-task-id-2"},
			RunID:      0,
			TakenUntil: tcclient.Time(time.Now().Add(10 * time.Minute)),
			Task: queue.TaskDefinitionResponse{
				Payload: json.RawMessage(`{
					"delay": 200,
					"function": "false",
					"argument": ""
				}`),
			},
		}),
	}, nil)
	q.On("ReportFailed", "my-task-id-2", "0").Once().Return(&queue.TaskStatusResponse{}, nil)

	// return a task, once
	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", mock.Anything).Once().Return(&queue.ClaimWorkResponse{
		Tasks: append(queue.ClaimWorkResponse{}.Tasks, taskClaim{
			Status:     queue.TaskStatusStructure{TaskID: "my-task-id-3"},
			RunID:      2,
			TakenUntil: tcclient.Time(time.Now().Add(10 * time.Minute)),
			Task: queue.TaskDefinitionResponse{
				Payload: json.RawMessage(`{
					"delay": 200,
					"function": "malformed-payload-after-start",
					"argument": "something bad in the payload"
				}`),
			},
		}),
	}, nil)
	q.On("ReportException", "my-task-id-3", "2", &queue.TaskExceptionRequest{
		Reason: "malformed-payload",
	}).Once().Return(&queue.TaskStatusResponse{}, nil)

	// return no tasks forever, and stop gracefully
	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", mock.Anything).Run(func(mock.Arguments) {
		w.StopGracefully()
	}).Return(&queue.ClaimWorkResponse{
		Tasks: append(queue.ClaimWorkResponse{}.Tasks),
	}, nil)
}

func TestWorkerProcessTasksConcurrently(t *testing.T) {
	// Set mock queue, server and worker
	q := client.MockQueue{}
	s := httptest.NewServer(&q)
	defer s.Close()
	defer q.AssertExpectations(t)
	w := setupTestWorker(t, s.URL, 3)
	defer w.Start()

	// Model the queue

	// return no task, once
	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", mock.Anything).Once().Return(&queue.ClaimWorkResponse{
		Tasks: append(queue.ClaimWorkResponse{}.Tasks),
	}, nil)

	// return 3 task, at once
	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", &queue.ClaimWorkRequest{
		Tasks:       3,
		WorkerGroup: "test-worker-group",
		WorkerID:    "test-worker-id",
	}).Once().Return(&queue.ClaimWorkResponse{
		Tasks: append(queue.ClaimWorkResponse{}.Tasks, taskClaim{
			Status:     queue.TaskStatusStructure{TaskID: "my-task-id-1"},
			RunID:      0,
			TakenUntil: tcclient.Time(time.Now().Add(10 * time.Minute)),
			Task: queue.TaskDefinitionResponse{
				Payload: json.RawMessage(`{
					"delay": 200,
					"function": "true",
					"argument": ""
				}`),
			},
		}, taskClaim{
			Status:     queue.TaskStatusStructure{TaskID: "my-task-id-2"},
			RunID:      0,
			TakenUntil: tcclient.Time(time.Now().Add(10 * time.Minute)),
			Task: queue.TaskDefinitionResponse{
				Payload: json.RawMessage(`{
					"delay": 500,
					"function": "false",
					"argument": ""
				}`),
			},
		}, taskClaim{
			Status:     queue.TaskStatusStructure{TaskID: "my-task-id-3"},
			RunID:      2,
			TakenUntil: tcclient.Time(time.Now().Add(10 * time.Minute)),
			Task: queue.TaskDefinitionResponse{
				Payload: json.RawMessage(`{
					"delay": 200,
					"function": "malformed-payload-after-start",
					"argument": "something bad in the payload"
				}`),
			},
		}),
	}, nil)
	q.On("ReportCompleted", "my-task-id-1", "0").Once().Return(&queue.TaskStatusResponse{}, nil)
	q.On("ReportFailed", "my-task-id-2", "0").Once().Return(&queue.TaskStatusResponse{}, nil)
	q.On("ReportException", "my-task-id-3", "2", &queue.TaskExceptionRequest{
		Reason: "malformed-payload",
	}).Once().Return(&queue.TaskStatusResponse{}, nil)

	// return no tasks forever, and stop gracefully
	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", mock.Anything).Run(func(mock.Arguments) {
		w.StopGracefully()
	}).Return(&queue.ClaimWorkResponse{
		Tasks: append(queue.ClaimWorkResponse{}.Tasks),
	}, nil)
}

func TestWorkerReclaimTask(t *testing.T) {
	// Set mock queue, server and worker
	q := client.MockQueue{}
	s := httptest.NewServer(&q)
	defer s.Close()
	defer q.AssertExpectations(t)
	w := setupTestWorker(t, s.URL, 1)
	defer w.Start()

	// Model the queue

	// return no task, once
	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", mock.Anything).Once().Return(&queue.ClaimWorkResponse{
		Tasks: append(queue.ClaimWorkResponse{}.Tasks),
	}, nil)

	// return 1 task, once
	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", &queue.ClaimWorkRequest{
		Tasks:       1,
		WorkerGroup: "test-worker-group",
		WorkerID:    "test-worker-id",
	}).Once().Return(&queue.ClaimWorkResponse{
		Tasks: append(queue.ClaimWorkResponse{}.Tasks, taskClaim{
			Status:     queue.TaskStatusStructure{TaskID: "my-task-id-1"},
			RunID:      0,
			TakenUntil: tcclient.Time(time.Now().Add(100 * time.Millisecond)),
			Task: queue.TaskDefinitionResponse{
				Payload: json.RawMessage(`{
					"delay": 3000,
					"function": "true",
					"argument": ""
				}`),
			},
		}),
	}, nil)
	q.On("ReclaimTask", "my-task-id-1", "0").Once().Return(&queue.TaskReclaimResponse{
		TakenUntil: tcclient.Time(time.Now().Add(100 * time.Millisecond)),
	}, nil)
	q.On("ReclaimTask", "my-task-id-1", "0").Once().Return(&queue.TaskReclaimResponse{
		TakenUntil: tcclient.Time(time.Now().Add(10 * time.Minute)),
	}, nil)
	q.On("ReportCompleted", "my-task-id-1", "0").Once().Return(&queue.TaskStatusResponse{}, nil)

	// return no tasks forever, and stop gracefully
	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", mock.Anything).Run(func(mock.Arguments) {
		w.StopGracefully()
	}).Return(&queue.ClaimWorkResponse{
		Tasks: append(queue.ClaimWorkResponse{}.Tasks),
	}, nil)
}

func TestWorkerTaskCanceled(t *testing.T) {
	// Set mock queue, server and worker
	q := client.MockQueue{}
	s := httptest.NewServer(&q)
	defer s.Close()
	defer q.AssertExpectations(t)
	w := setupTestWorker(t, s.URL, 1)
	defer w.Start()

	// Model the queue

	// return no task, once
	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", mock.Anything).Once().Return(&queue.ClaimWorkResponse{
		Tasks: append(queue.ClaimWorkResponse{}.Tasks),
	}, nil)

	// return 1 task, once
	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", &queue.ClaimWorkRequest{
		Tasks:       1,
		WorkerGroup: "test-worker-group",
		WorkerID:    "test-worker-id",
	}).Once().Return(&queue.ClaimWorkResponse{
		Tasks: append(queue.ClaimWorkResponse{}.Tasks, taskClaim{
			Status:     queue.TaskStatusStructure{TaskID: "my-task-id-1"},
			RunID:      0,
			TakenUntil: tcclient.Time(time.Now().Add(100 * time.Millisecond)),
			Task: queue.TaskDefinitionResponse{
				Payload: json.RawMessage(`{
					"delay": 1500,
					"function": "true",
					"argument": ""
				}`),
			},
		}),
	}, nil)
	q.On("ReclaimTask", "my-task-id-1", "0").Once().Return((*queue.TaskReclaimResponse)(nil), httpbackoff.BadHttpResponseCode{
		HttpResponseCode: 409,
		Message:          "task canceled",
	})

	// return no tasks forever, and stop gracefully
	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", mock.Anything).Run(func(mock.Arguments) {
		w.StopGracefully()
	}).Return(&queue.ClaimWorkResponse{
		Tasks: append(queue.ClaimWorkResponse{}.Tasks),
	}, nil)
}

func TestWorkerStopNow(t *testing.T) {
	// Set mock queue, server and worker
	q := client.MockQueue{}
	s := httptest.NewServer(&q)
	defer s.Close()
	defer q.AssertExpectations(t)
	w := setupTestWorker(t, s.URL, 1)
	defer w.Start()

	// Model the queue

	// return no task, once
	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", mock.Anything).Once().Return(&queue.ClaimWorkResponse{
		Tasks: append(queue.ClaimWorkResponse{}.Tasks),
	}, nil)

	// return 1 task, once
	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", &queue.ClaimWorkRequest{
		Tasks:       1,
		WorkerGroup: "test-worker-group",
		WorkerID:    "test-worker-id",
	}).Once().Run(func(mock.Arguments) {
		w.StopNow()
	}).Return(&queue.ClaimWorkResponse{
		Tasks: append(queue.ClaimWorkResponse{}.Tasks, taskClaim{
			Status:     queue.TaskStatusStructure{TaskID: "my-task-id-1"},
			RunID:      0,
			TakenUntil: tcclient.Time(time.Now().Add(10 * time.Minute)),
			Task: queue.TaskDefinitionResponse{
				Payload: json.RawMessage(`{
					"delay": 200,
					"function": "true",
					"argument": ""
				}`),
			},
		}),
	}, nil)
	q.On("ReportException", "my-task-id-1", "0", &queue.TaskExceptionRequest{
		Reason: "worker-shutdown",
	}).Once().Return(&queue.TaskStatusResponse{}, nil)

	// return no tasks forever, and stop gracefully
	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", mock.Anything).Run(func(mock.Arguments) {
		w.StopGracefully()
	}).Return(&queue.ClaimWorkResponse{
		Tasks: append(queue.ClaimWorkResponse{}.Tasks),
	}, nil)
}
