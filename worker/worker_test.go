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
	"github.com/taskcluster/taskcluster-client-go/tcqueue"
	_ "github.com/taskcluster/taskcluster-worker/engines/mock"
	_ "github.com/taskcluster/taskcluster-worker/plugins/success"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
)

func setupTestWorker(t *testing.T, queueBaseURL string, concurrency int) *Worker {
	tempFolder, _ := json.Marshal(path.Join(os.TempDir(), slugid.Nice()))
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
		"temporaryFolder": ` + string(tempFolder) + `,
		"minimumDiskSpace": 0,
		"minimumMemory": 0,
		"monitor": {"type": "mock", "panicOnError": true},
		"credentials": {
			"clientId": "my-test-client-id",
			"accessToken": "my-super-secret-access-token"
		},
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
	w.queueBaseURL = queueBaseURL
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
	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", mock.Anything).Twice().Return(&tcqueue.ClaimWorkResponse{
		Tasks: append(tcqueue.ClaimWorkResponse{}.Tasks),
	}, nil)

	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", mock.Anything).Once().Run(func(args mock.Arguments) {
		w.StopGracefully()
	}).Return(&tcqueue.ClaimWorkResponse{
		Tasks: append(tcqueue.ClaimWorkResponse{}.Tasks),
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
	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", mock.Anything).Once().Return(&tcqueue.ClaimWorkResponse{
		Tasks: append(tcqueue.ClaimWorkResponse{}.Tasks),
	}, nil)

	// return a task, once
	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", mock.Anything).Once().Return(&tcqueue.ClaimWorkResponse{
		Tasks: append(tcqueue.ClaimWorkResponse{}.Tasks, taskClaim{
			Status:     tcqueue.TaskStatusStructure{TaskID: "my-task-id-1"},
			RunID:      0,
			TakenUntil: tcclient.Time(time.Now().Add(10 * time.Minute)),
			Task: tcqueue.TaskDefinitionResponse{
				Payload: json.RawMessage(`{
					"delay": 200,
					"function": "true",
					"argument": ""
				}`),
			},
		}),
	}, nil)
	q.On("ReportCompleted", "my-task-id-1", "0").Once().Return(&tcqueue.TaskStatusResponse{}, nil)

	// return a task, once
	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", mock.Anything).Once().Return(&tcqueue.ClaimWorkResponse{
		Tasks: append(tcqueue.ClaimWorkResponse{}.Tasks, taskClaim{
			Status:     tcqueue.TaskStatusStructure{TaskID: "my-task-id-2"},
			RunID:      0,
			TakenUntil: tcclient.Time(time.Now().Add(10 * time.Minute)),
			Task: tcqueue.TaskDefinitionResponse{
				Payload: json.RawMessage(`{
					"delay": 200,
					"function": "false",
					"argument": ""
				}`),
			},
		}),
	}, nil)
	q.On("ReportFailed", "my-task-id-2", "0").Once().Return(&tcqueue.TaskStatusResponse{}, nil)

	// return a task, once
	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", mock.Anything).Once().Return(&tcqueue.ClaimWorkResponse{
		Tasks: append(tcqueue.ClaimWorkResponse{}.Tasks, taskClaim{
			Status:     tcqueue.TaskStatusStructure{TaskID: "my-task-id-3"},
			RunID:      2,
			TakenUntil: tcclient.Time(time.Now().Add(10 * time.Minute)),
			Task: tcqueue.TaskDefinitionResponse{
				Payload: json.RawMessage(`{
					"delay": 200,
					"function": "malformed-payload-after-start",
					"argument": "something bad in the payload"
				}`),
			},
		}),
	}, nil)
	q.On("ReportException", "my-task-id-3", "2", &tcqueue.TaskExceptionRequest{
		Reason: "malformed-payload",
	}).Once().Return(&tcqueue.TaskStatusResponse{}, nil)

	// return no tasks forever, and stop gracefully
	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", mock.Anything).Run(func(mock.Arguments) {
		w.StopGracefully()
	}).Return(&tcqueue.ClaimWorkResponse{
		Tasks: append(tcqueue.ClaimWorkResponse{}.Tasks),
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
	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", mock.Anything).Once().Return(&tcqueue.ClaimWorkResponse{
		Tasks: append(tcqueue.ClaimWorkResponse{}.Tasks),
	}, nil)

	// return 3 task, at once
	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", &tcqueue.ClaimWorkRequest{
		Tasks:       3,
		WorkerGroup: "test-worker-group",
		WorkerID:    "test-worker-id",
	}).Once().Return(&tcqueue.ClaimWorkResponse{
		Tasks: append(tcqueue.ClaimWorkResponse{}.Tasks, taskClaim{
			Status:     tcqueue.TaskStatusStructure{TaskID: "my-task-id-1"},
			RunID:      0,
			TakenUntil: tcclient.Time(time.Now().Add(10 * time.Minute)),
			Task: tcqueue.TaskDefinitionResponse{
				Payload: json.RawMessage(`{
					"delay": 200,
					"function": "true",
					"argument": ""
				}`),
			},
		}, taskClaim{
			Status:     tcqueue.TaskStatusStructure{TaskID: "my-task-id-2"},
			RunID:      0,
			TakenUntil: tcclient.Time(time.Now().Add(10 * time.Minute)),
			Task: tcqueue.TaskDefinitionResponse{
				Payload: json.RawMessage(`{
					"delay": 500,
					"function": "false",
					"argument": ""
				}`),
			},
		}, taskClaim{
			Status:     tcqueue.TaskStatusStructure{TaskID: "my-task-id-3"},
			RunID:      2,
			TakenUntil: tcclient.Time(time.Now().Add(10 * time.Minute)),
			Task: tcqueue.TaskDefinitionResponse{
				Payload: json.RawMessage(`{
					"delay": 200,
					"function": "malformed-payload-after-start",
					"argument": "something bad in the payload"
				}`),
			},
		}),
	}, nil)
	q.On("ReportCompleted", "my-task-id-1", "0").Once().Return(&tcqueue.TaskStatusResponse{}, nil)
	q.On("ReportFailed", "my-task-id-2", "0").Once().Return(&tcqueue.TaskStatusResponse{}, nil)
	q.On("ReportException", "my-task-id-3", "2", &tcqueue.TaskExceptionRequest{
		Reason: "malformed-payload",
	}).Once().Return(&tcqueue.TaskStatusResponse{}, nil)

	// return no tasks forever, and stop gracefully
	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", mock.Anything).Run(func(mock.Arguments) {
		w.StopGracefully()
	}).Return(&tcqueue.ClaimWorkResponse{
		Tasks: append(tcqueue.ClaimWorkResponse{}.Tasks),
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
	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", mock.Anything).Once().Return(&tcqueue.ClaimWorkResponse{
		Tasks: append(tcqueue.ClaimWorkResponse{}.Tasks),
	}, nil)

	// return 1 task, once
	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", &tcqueue.ClaimWorkRequest{
		Tasks:       1,
		WorkerGroup: "test-worker-group",
		WorkerID:    "test-worker-id",
	}).Once().Return(&tcqueue.ClaimWorkResponse{
		Tasks: append(tcqueue.ClaimWorkResponse{}.Tasks, taskClaim{
			Status:     tcqueue.TaskStatusStructure{TaskID: "my-task-id-1"},
			RunID:      0,
			TakenUntil: tcclient.Time(time.Now().Add(100 * time.Millisecond)),
			Task: tcqueue.TaskDefinitionResponse{
				Payload: json.RawMessage(`{
					"delay": 3000,
					"function": "true",
					"argument": ""
				}`),
			},
		}),
	}, nil)
	q.On("ReclaimTask", "my-task-id-1", "0").Once().Return(&tcqueue.TaskReclaimResponse{
		TakenUntil: tcclient.Time(time.Now().Add(100 * time.Millisecond)),
	}, nil)
	q.On("ReclaimTask", "my-task-id-1", "0").Once().Return(&tcqueue.TaskReclaimResponse{
		TakenUntil: tcclient.Time(time.Now().Add(10 * time.Minute)),
	}, nil)
	q.On("ReportCompleted", "my-task-id-1", "0").Once().Return(&tcqueue.TaskStatusResponse{}, nil)

	// return no tasks forever, and stop gracefully
	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", mock.Anything).Run(func(mock.Arguments) {
		w.StopGracefully()
	}).Return(&tcqueue.ClaimWorkResponse{
		Tasks: append(tcqueue.ClaimWorkResponse{}.Tasks),
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
	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", mock.Anything).Once().Return(&tcqueue.ClaimWorkResponse{
		Tasks: append(tcqueue.ClaimWorkResponse{}.Tasks),
	}, nil)

	// return 1 task, once
	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", &tcqueue.ClaimWorkRequest{
		Tasks:       1,
		WorkerGroup: "test-worker-group",
		WorkerID:    "test-worker-id",
	}).Once().Return(&tcqueue.ClaimWorkResponse{
		Tasks: append(tcqueue.ClaimWorkResponse{}.Tasks, taskClaim{
			Status:     tcqueue.TaskStatusStructure{TaskID: "my-task-id-1"},
			RunID:      0,
			TakenUntil: tcclient.Time(time.Now().Add(100 * time.Millisecond)),
			Task: tcqueue.TaskDefinitionResponse{
				Payload: json.RawMessage(`{
					"delay": 1500,
					"function": "true",
					"argument": ""
				}`),
			},
		}),
	}, nil)
	q.On("ReclaimTask", "my-task-id-1", "0").Once().Return((*tcqueue.TaskReclaimResponse)(nil), httpbackoff.BadHttpResponseCode{
		HttpResponseCode: 409,
		Message:          "task canceled",
	})

	// return no tasks forever, and stop gracefully
	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", mock.Anything).Run(func(mock.Arguments) {
		w.StopGracefully()
	}).Return(&tcqueue.ClaimWorkResponse{
		Tasks: append(tcqueue.ClaimWorkResponse{}.Tasks),
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
	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", mock.Anything).Once().Return(&tcqueue.ClaimWorkResponse{
		Tasks: append(tcqueue.ClaimWorkResponse{}.Tasks),
	}, nil)

	// return 1 task, once
	q.On("ClaimWork", "test-provisioner-id", "test-worker-type", &tcqueue.ClaimWorkRequest{
		Tasks:       1,
		WorkerGroup: "test-worker-group",
		WorkerID:    "test-worker-id",
	}).Once().Return(&tcqueue.ClaimWorkResponse{
		Tasks: append(tcqueue.ClaimWorkResponse{}.Tasks, taskClaim{
			Status:     tcqueue.TaskStatusStructure{TaskID: "my-task-id-1"},
			RunID:      0,
			TakenUntil: tcclient.Time(time.Now().Add(10 * time.Minute)),
			Task: tcqueue.TaskDefinitionResponse{
				Payload: json.RawMessage(`{
					"delay": 50,
					"function": "stopNow-sleep",
					"argument": ""
				}`),
			},
		}),
	}, nil)
	q.On("ReportException", "my-task-id-1", "0", &tcqueue.TaskExceptionRequest{
		Reason: "worker-shutdown",
	}).Once().Return(&tcqueue.TaskStatusResponse{}, nil)
}
