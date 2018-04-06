package fakequeue

import (
	"encoding/json"
	"reflect"
	"time"

	tcclient "github.com/taskcluster/taskcluster-client-go"
	"github.com/taskcluster/taskcluster-client-go/tcqueue"
)

const (
	statusUnscheduled = "unscheduled"
	statusPending     = "pending"
	statusRunning     = "running"
	statusCompleted   = "completed"
	statusFailed      = "failed"
	statusException   = "exception"
)

// setTaskDefaults creates a TaskDefinitionResponse from a TaskDefinitionRequest
// with defaults applies as taskcluster-queue would
func setTaskDefaults(taskID string, task *tcqueue.TaskDefinitionRequest) tcqueue.TaskDefinitionResponse {
	expires := task.Expires
	if expires == (tcclient.Time{}) {
		expires = tcclient.Time(time.Time(task.Created).Add(365 * 24 * 60 * 60 * time.Second))
	}
	extra := json.RawMessage(`{}`)
	if task.Extra != nil {
		extra = task.Extra
	}
	priority := "normal"
	if task.Priority != "" {
		priority = task.Priority
	}
	requires := "all-completed"
	if task.Requires != "" {
		requires = task.Requires
	}
	retries := 5
	if task.Retries != 0 {
		retries = int(task.Retries)
	}
	schedulerID := "-"
	if task.SchedulerID != "" {
		schedulerID = task.SchedulerID
	}
	var tags map[string]string
	if task.Tags != nil {
		tags = make(map[string]string)
		for k, v := range task.Tags {
			tags[k] = v
		}
	}
	taskGroupID := taskID
	if task.TaskGroupID != "" {
		taskGroupID = task.TaskGroupID
	}
	return tcqueue.TaskDefinitionResponse{
		Created:       task.Created,
		Deadline:      task.Deadline,
		Dependencies:  task.Dependencies,
		Expires:       expires,
		Extra:         extra,
		Metadata:      task.Metadata,
		Payload:       task.Payload,
		Priority:      priority,
		ProvisionerID: task.ProvisionerID,
		Requires:      requires,
		Retries:       int64(retries),
		Routes:        task.Routes,
		SchedulerID:   schedulerID,
		Scopes:        task.Scopes,
		Tags:          tags,
		TaskGroupID:   taskGroupID,
		WorkerType:    task.WorkerType,
	}
}

func isJSONEqual(val1, val2 interface{}) bool {
	d1, _ := json.Marshal(val1)
	d2, _ := json.Marshal(val2)
	var v1, v2 interface{}
	_ = json.Unmarshal(d1, &v1)
	_ = json.Unmarshal(d2, &v2)
	return reflect.DeepEqual(v1, v2)
}

// fakeCredentials as defined in tcqueue.ClaimWorkResponse.Tasks[i].Credentials
var fakeCredentials = struct {
	AccessToken string `json:"accessToken"`
	Certificate string `json:"certificate"`
	ClientID    string `json:"clientId"`
}{
	ClientID:    "some-super-fake-client-for-use-in-responses",
	AccessToken: "some-super-fake-secret-for-use-in-responses",
}

// reconcileTasks will reconcile task states, expiring claims, enforcing
// deadlines, deleting expired tasks/artifacts, and scheduling unblocked tasks.
func reconcileTasks(tasks map[string]*task) {
	// Delete tasks with expires < now
	for taskID, t := range tasks {
		if time.Time(t.task.Expires).Before(time.Now()) {
			debug("expiring taskId: %s", taskID)
			delete(tasks, taskID)
		}
	}

	// Delete artifacts with expires < now
	for taskID, t := range tasks {
		for runID, artifacts := range t.artifacts {
			for name, artifact := range artifacts {
				if artifact.Expires.Before(time.Now()) {
					debug("expiring artifact for taskId: %s, runId: %d, name: %s", taskID, runID, name)
					delete(artifacts, name)
				}
			}
		}
	}

	// Resolve tasks with deadline < now
	for taskID, t := range tasks {
		// Skip resolved tasks
		if t.status.State == statusCompleted ||
			t.status.State == statusFailed ||
			t.status.State == statusException {
			continue
		}
		// Skip tasks with deadline > now
		if !time.Time(t.status.Deadline).Before(time.Now()) {
			continue
		}

		if t.status.State == statusUnscheduled {
			// Add new resolved run
			debug("deadline-exceeded in taskId: %s", t.status.TaskID)
			t.status.Runs = append(t.status.Runs, run{
				ReasonCreated:  "exception",
				ReasonResolved: "deadline-exceeded",
				Resolved:       tcclient.Time(time.Now()),
				RunID:          0,
				Scheduled:      tcclient.Time(time.Now()),
				State:          statusException,
			})
			t.artifacts = append(t.artifacts, make(map[string]artifact))
			t.status.State = statusException
		} else {
			// Resolve latest run
			runID := len(t.status.Runs) - 1
			debug("deadline-exceeded in taskId: %s, runId: %d", taskID, runID)
			t.status.Runs[runID].State = statusException
			t.status.Runs[runID].ReasonResolved = "deadline-exceeded"
			t.status.Runs[runID].Resolved = tcclient.Time(time.Now())
			t.status.State = statusException
		}
	}

	// Resolve runs with takenUntil < now
	for taskID, t := range tasks {
		// Skip tasks not running
		if t.status.State != statusRunning {
			continue
		}

		// Skip tasks with takenUntil > now
		runID := len(t.status.Runs) - 1
		if !time.Time(t.status.Runs[runID].TakenUntil).Before(time.Now()) {
			continue
		}

		// Resolve the run
		debug("takenUntil expired for taskId: %s, runId: %d", taskID, runID)
		t.status.Runs[runID].ReasonResolved = "claim-expired"
		t.status.Runs[runID].Resolved = tcclient.Time(time.Now())
		t.status.Runs[runID].State = statusException
		t.status.State = statusException

		// Add new run if we have retries left
		if t.status.RetriesLeft > 0 {
			t.status.RetriesLeft--
			debug("scheduling a retry of taskId: %s", taskID)
			t.status.Runs = append(t.status.Runs, run{
				ReasonCreated: "retry",
				RunID:         int64(len(t.status.Runs)),
				Scheduled:     tcclient.Time(time.Now()),
				State:         statusPending,
			})
			t.artifacts = append(t.artifacts, make(map[string]artifact))
			t.status.State = statusPending
		}
	}

	// Schedule all tasks whose dependencies have been resolved
	for _, t := range tasks {
		// Skip anything that's already scheduled
		if t.status.State != statusUnscheduled {
			continue
		}

		// Check if all dependencies is completed or resolved
		isCompleted := true
		isResolved := true
		for _, dep := range t.task.Dependencies {
			s := tasks[dep].status.State
			isCompleted = isCompleted && s == statusCompleted
			isResolved = isResolved && (s == statusCompleted || s == statusFailed || s == statusException)
		}
		if isCompleted || (t.task.Requires == "all-resolved" && isResolved) {
			// Schedule task
			debug("scheduling taskId: %s", t.status.TaskID)
			t.status.State = statusPending
			t.status.Runs = append(t.status.Runs, run{
				ReasonCreated: "scheduled",
				RunID:         int64(len(t.status.Runs)),
				Scheduled:     tcclient.Time(time.Now()),
				State:         statusPending,
			})
			t.artifacts = append(t.artifacts, make(map[string]artifact))
		}
	}
}
