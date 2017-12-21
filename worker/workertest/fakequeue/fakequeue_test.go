package fakequeue

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/taskcluster/slugid-go/slugid"
	tcclient "github.com/taskcluster/taskcluster-client-go"
	"github.com/taskcluster/taskcluster-client-go/queue"
)

const (
	testProvisionerID = "dummy-test-provisioner"
	testWorkerType    = "dummy-test-worker-317"
)

func TestFakeQueueWithClaimWork(t *testing.T) {
	s := httptest.NewServer(New())
	defer s.Close()

	q := queue.New(&tcclient.Credentials{})
	q.BaseURL = s.URL

	debug("### Creating task")
	taskID := slugid.Nice()
	task := queue.TaskDefinitionRequest{
		ProvisionerID: testProvisionerID,
		WorkerType:    testWorkerType,
		Created:       tcclient.Time(time.Now()),
		Deadline:      tcclient.Time(time.Now().Add(60 * time.Minute)),
		Payload:       json.RawMessage(`{}`),
	}
	task.Metadata.Name = "test task"
	task.Metadata.Description = "Some test task to see if fake queue works"
	task.Metadata.Source = "https://github.com/taskcluster/taskcluster-worker/tree/master/worker/workertest/fakequeue/fakequeue_test.go"
	task.Metadata.Owner = "jonasfj@mozilla.com"
	_, err := q.CreateTask(taskID, &task)
	assert.NoError(t, err, "failed to create task")

	debug("### Claim work")
	claims, err := q.ClaimWork(testProvisionerID, testWorkerType, &queue.ClaimWorkRequest{
		Tasks:       5,
		WorkerGroup: "test-worker-group",
		WorkerID:    "test-worker-42",
	})
	assert.NoError(t, err, "failed to claim work")
	assert.True(t, len(claims.Tasks) == 1, "Expected 1 task")
	assert.True(t, claims.Tasks[0].Status.TaskID == taskID, "expected taskID")
	assert.True(t, claims.Tasks[0].RunID == 0, "expected runID = 0")

	debug("### Reclaim Task")
	_, err = q.ReclaimTask(taskID, "0")
	assert.NoError(t, err, "failed to reclaim")

	debug("### Reclaim Task (again)")
	_, err = q.ReclaimTask(taskID, "0")
	assert.NoError(t, err, "failed to reclaim (again)")

	debug("### Report completed")
	_, err = q.ReportCompleted(taskID, "0")
	assert.NoError(t, err, "failed to report completed")

	debug("### Report completed (again)")
	_, err = q.ReportCompleted(taskID, "0")
	assert.NoError(t, err, "failed to report completed (again)")

	debug("### Get task status")
	status, err := q.Status(taskID)
	assert.NoError(t, err, "failed to get tasks status")
	assert.True(t, status.Status.State == "completed", "Expected task to be completed")
}

func TestFakeQueueWithClaimTask(t *testing.T) {
	s := httptest.NewServer(New())
	defer s.Close()

	q := queue.New(&tcclient.Credentials{})
	q.BaseURL = s.URL

	debug("### Creating task")
	taskID := slugid.Nice()
	task := queue.TaskDefinitionRequest{
		ProvisionerID: testProvisionerID,
		WorkerType:    testWorkerType,
		Created:       tcclient.Time(time.Now()),
		Deadline:      tcclient.Time(time.Now().Add(60 * time.Minute)),
		Payload:       json.RawMessage(`{}`),
	}
	task.Metadata.Name = "test task"
	task.Metadata.Description = "Some test task to see if fake queue works"
	task.Metadata.Source = "https://github.com/taskcluster/taskcluster-worker/tree/master/worker/workertest/fakequeue/fakequeue_test.go"
	task.Metadata.Owner = "jonasfj@mozilla.com"
	_, err := q.CreateTask(taskID, &task)
	assert.NoError(t, err, "failed to create task")

	debug("### Claim work")
	claim, err := q.ClaimTask(taskID, "0", &queue.TaskClaimRequest{
		WorkerGroup: "test-worker-group",
		WorkerID:    "test-worker-42",
	})
	assert.NoError(t, err, "failed to claim work")
	assert.True(t, claim.Status.TaskID == taskID, "expected taskID")
	assert.True(t, claim.RunID == 0, "expected runID = 0")

	debug("### Reclaim Task")
	_, err = q.ReclaimTask(taskID, "0")
	assert.NoError(t, err, "failed to reclaim")

	debug("### Reclaim Task (again)")
	_, err = q.ReclaimTask(taskID, "0")
	assert.NoError(t, err, "failed to reclaim (again)")

	debug("### Report completed")
	_, err = q.ReportCompleted(taskID, "0")
	assert.NoError(t, err, "failed to report completed")

	debug("### Report completed (again)")
	_, err = q.ReportCompleted(taskID, "0")
	assert.NoError(t, err, "failed to report completed (again)")

	debug("### Get task status")
	status, err := q.Status(taskID)
	assert.NoError(t, err, "failed to get tasks status")
	assert.True(t, status.Status.State == "completed", "Expected task to be completed")
}
