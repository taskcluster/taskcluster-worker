package fakequeue

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"time"

	tcclient "github.com/taskcluster/taskcluster-client-go"
	"github.com/taskcluster/taskcluster-client-go/tcqueue"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
)

// FakeQueue is a taskcluster-queue implementation with certain limitations:
//  * No validation of authentication or authenorization
//  * Data stored in-memory
// The FakeQueue supports the following end-points:
//  * task
//  * status
//  * createTask
//  * cancelTask
//  * claimWork
//  * claimTask
//  * reclaimTask
//  * reportCompleted
//  * reportFailed
//  * reportException
//  * createArtifact
//  * getArtifact
//  * getLatestArtifact
//  * listLatestArtifact
//  * pendingTasks
type FakeQueue struct {
	m     sync.Mutex
	c     sync.Cond
	tasks map[string]*task
}

// New returns a new FakeQueue
func New() *FakeQueue {
	return &FakeQueue{}
}

type task struct {
	status    tcqueue.TaskStatusStructure
	task      tcqueue.TaskDefinitionResponse
	artifacts []map[string]artifact
}

type run struct { // as defined in tcqueue.TaskStatusStructure.Runs
	ReasonCreated  string        `json:"reasonCreated"`
	ReasonResolved string        `json:"reasonResolved,omitempty"`
	Resolved       tcclient.Time `json:"resolved,omitempty"`
	RunID          int64         `json:"runId"`
	Scheduled      tcclient.Time `json:"scheduled"`
	Started        tcclient.Time `json:"started,omitempty"`
	State          string        `json:"state"`
	TakenUntil     tcclient.Time `json:"takenUntil,omitempty"`
	WorkerGroup    string        `json:"workerGroup,omitempty"`
	WorkerID       string        `json:"workerId,omitempty"`
}

const (
	storageTypeS3        = "s3"
	storageTypeAzure     = "azure"
	storageTypeReference = "reference"
	storageTypeError     = "error"
)

type artifact struct {
	StorageType     string    `json:"storageType"`
	Expires         time.Time `json:"expires"`
	ContentType     string    `json:"contentType,omitempty"`
	URL             string    `json:"url,omitempty"`
	Reason          string    `json:"reason,omitempty"`
	Message         string    `json:"message,omitempty"`
	Data            []byte    `json:"-"`
	ContentEncoding string    `json:"-"`
}

func (q *FakeQueue) initAndLock() {
	q.m.Lock()
	if q.c.L == nil {
		q.c.L = &q.m
		q.tasks = make(map[string]*task)
	}
}

var (
	patternTask                = regexp.MustCompile("^/task/([a-zA-Z0-9_-]{22})$")
	patternCreateTask          = regexp.MustCompile("^/task/([a-zA-Z0-9_-]{22})$")
	patternStatus              = regexp.MustCompile("^/task/([a-zA-Z0-9_-]{22})/status$")
	patternCancelTask          = regexp.MustCompile("^/task/([a-zA-Z0-9_-]{22})/cancel$")
	patternClaimTask           = regexp.MustCompile("^/task/([a-zA-Z0-9_-]{22})/runs/([0-9]+)/claim$")
	patternReclaimTask         = regexp.MustCompile("^/task/([a-zA-Z0-9_-]{22})/runs/([0-9]+)/reclaim$")
	patternReportCompleted     = regexp.MustCompile("^/task/([a-zA-Z0-9_-]{22})/runs/([0-9]+)/completed$")
	patternReportFailed        = regexp.MustCompile("^/task/([a-zA-Z0-9_-]{22})/runs/([0-9]+)/failed$")
	patternReportException     = regexp.MustCompile("^/task/([a-zA-Z0-9_-]{22})/runs/([0-9]+)/exception$")
	patternCreateArtifact      = regexp.MustCompile("^/task/([a-zA-Z0-9_-]{22})/runs/([0-9]+)/artifacts/(.*)$")
	patternGetArtifact         = regexp.MustCompile("^/task/([a-zA-Z0-9_-]{22})/runs/([0-9]+)/artifacts/(.*)$")
	patternListArtifacts       = regexp.MustCompile("^/task/([a-zA-Z0-9_-]{22})/runs/([0-9]+)/artifacts$")
	patternGetLatestArtifact   = regexp.MustCompile("^/task/([a-zA-Z0-9_-]{22})/artifacts/(.*)$")
	patternListLatestArtifacts = regexp.MustCompile("^/task/([a-zA-Z0-9_-]{22})/artifacts$")
	patternClaimWork           = regexp.MustCompile("^/claim-work/([a-zA-Z0-9_-]{1,22})/([a-zA-Z0-9_-]{1,22})$")
	patternPendingTasks        = regexp.MustCompile("^/pending/([a-zA-Z0-9_-]{1,22})/([a-zA-Z0-9_-]{1,22})$")
	patternPing                = regexp.MustCompile("^/ping$")
	patternArtifactPutURL      = regexp.MustCompile("^/internal/task/([a-zA-Z0-9_-]{22})/runs/([0-9]+)/put-artifact/(.*)$")
)

func (q *FakeQueue) task(taskID string) interface{} {
	if t, ok := q.tasks[taskID]; ok {
		return t.task
	}
	return resourceNotFoundError
}

func (q *FakeQueue) status(taskID string) interface{} {
	if t, ok := q.tasks[taskID]; ok {
		return tcqueue.TaskStatusResponse{Status: t.status}
	}
	return resourceNotFoundError
}

func (q *FakeQueue) createTask(taskID string, payload *tcqueue.TaskDefinitionRequest) interface{} {
	def := setTaskDefaults(taskID, payload)

	// check that dependencies have been created
	for _, dep := range def.Dependencies {
		if _, ok := q.tasks[dep]; !ok {
			return restError{
				StatusCode: http.StatusBadRequest,
				Code:       "BadPayload",
				Message:    "Task depends on a task that doesn't exist!",
			}
		}
	}

	// Handle case where the task exists
	if t, ok := q.tasks[taskID]; ok {
		if isJSONEqual(t.task, def) {
			return tcqueue.TaskStatusResponse{Status: t.status}
		}
		return restError{
			StatusCode: http.StatusConflict,
			Code:       "RequestConflict",
			Message:    "Task with given taskID already exists with a different definition",
		}
	}

	// Insert task in database
	q.tasks[taskID] = &task{
		task: def,
		status: tcqueue.TaskStatusStructure{
			Deadline:      def.Deadline,
			Expires:       def.Expires,
			ProvisionerID: def.ProvisionerID,
			RetriesLeft:   def.Retries,
			SchedulerID:   def.SchedulerID,
			State:         statusUnscheduled,
			TaskGroupID:   def.TaskGroupID,
			TaskID:        taskID,
			WorkerType:    def.WorkerType,
		},
	}
	// Schedule all tasks ready to be scheduled
	reconcileTasks(q.tasks)

	return tcqueue.TaskStatusResponse{Status: q.tasks[taskID].status}
}

func (q *FakeQueue) cancelTask(taskID string) interface{} {
	t, ok := q.tasks[taskID]
	if !ok {
		return resourceNotFoundError
	}

	// Added dummy run, if unscheduled
	if t.status.State == statusUnscheduled {
		t.status.Runs = append(t.status.Runs, run{
			RunID:          0,
			ReasonCreated:  "exception",
			ReasonResolved: "canceled",
			Resolved:       tcclient.Time(time.Now()),
			Scheduled:      tcclient.Time(time.Now()),
			State:          statusException,
		})
		t.artifacts = append(t.artifacts, make(map[string]artifact))
		t.status.State = statusException
	}

	// If pending or running we cancel the run
	if t.status.State == statusPending || t.status.State == statusRunning {
		runID := len(t.status.Runs) - 1
		t.status.Runs[runID].State = statusException
		t.status.Runs[runID].Resolved = tcclient.Time(time.Now())
		t.status.Runs[runID].ReasonResolved = "canceled"
		t.status.State = statusException
	}

	if t.status.State != statusException || t.status.Runs[len(t.status.Runs)-1].ReasonResolved != "canceled" {
		return restError{
			StatusCode: http.StatusConflict,
			Code:       "RequestConflict",
			Message:    "Task already resolved something other than canceled",
		}
	}

	return tcqueue.TaskStatusResponse{Status: t.status}
}

func (q *FakeQueue) claimWork(provisionerID, workerType string, payload *tcqueue.ClaimWorkRequest, w http.ResponseWriter) interface{} {
	var finished atomics.Once
	var result tcqueue.ClaimWorkResponse
	go func() {
		q.m.Lock()
		defer q.m.Unlock()
		for !finished.IsDone() {
			// schedule all tasks that may have expired, etc
			reconcileTasks(q.tasks)

			// Find tasks
			for _, t := range q.tasks {
				// Skip tasks that aren't pending or have wrong provisionerID/workerType
				if t.status.State != statusPending || t.task.ProvisionerID != provisionerID || t.task.WorkerType != workerType {
					continue
				}

				// Take this run
				runID := len(t.status.Runs) - 1
				t.status.Runs[runID].Started = tcclient.Time(time.Now())
				t.status.Runs[runID].WorkerGroup = payload.WorkerGroup
				t.status.Runs[runID].WorkerID = payload.WorkerID
				t.status.Runs[runID].TakenUntil = tcclient.Time(time.Now().Add(5 * time.Minute))
				t.status.Runs[runID].State = statusRunning
				t.status.State = statusRunning

				// Add claim to result
				result.Tasks = append(result.Tasks, struct {
					Credentials struct {
						AccessToken string `json:"accessToken"`
						Certificate string `json:"certificate"`
						ClientID    string `json:"clientId"`
					} `json:"credentials"`
					RunID       int64                          `json:"runId"`
					Status      tcqueue.TaskStatusStructure    `json:"status"`
					TakenUntil  tcclient.Time                  `json:"takenUntil"`
					Task        tcqueue.TaskDefinitionResponse `json:"task"`
					WorkerGroup string                         `json:"workerGroup"`
					WorkerID    string                         `json:"workerId"`
				}{
					Credentials: fakeCredentials,
					RunID:       int64(runID),
					Status:      t.status,
					TakenUntil:  t.status.Runs[runID].TakenUntil,
					Task:        t.task,
					WorkerGroup: payload.WorkerGroup,
					WorkerID:    payload.WorkerID,
				})

				// Stop looping through tasks if we have enough
				if len(result.Tasks) >= int(payload.Tasks) {
					break
				}
			}

			// If we found any pending tasks that we now have a claim for then we'r
			// done
			if len(result.Tasks) > 0 {
				finished.Do(nil)
				break
			}

			// Temporarily release lock and wait for signal that something changed
			q.c.Wait()
		}
	}()

	// Wait for canceled, use nil channel if not supported
	var canceled <-chan bool
	if notifier, ok := w.(http.CloseNotifier); ok {
		canceled = notifier.CloseNotify()
	}

	// Wait for done
	q.m.Unlock()
	select {
	case <-canceled:
	case <-time.After(5 * time.Second):
	case <-finished.Done():
	}
	q.m.Lock()

	finished.Do(nil)
	return result
}

func (q *FakeQueue) claimTask(taskID string, runID int, payload *tcqueue.TaskClaimRequest) interface{} {
	t, ok := q.tasks[taskID]
	if !ok || len(t.status.Runs) <= runID {
		return resourceNotFoundError
	}

	if t.status.Runs[runID].State == statusRunning {
		if payload.WorkerGroup != t.status.Runs[runID].WorkerGroup || payload.WorkerID != t.status.Runs[runID].WorkerID {
			return restError{
				StatusCode: http.StatusConflict,
				Code:       "RequestConflict",
				Message:    "run claimed by another worker",
			}
		}
	} else if t.status.Runs[runID].State != statusPending {
		return restError{
			StatusCode: http.StatusConflict,
			Code:       "RequestConflict",
			Message:    "run not pending",
		}
	}

	// Set run properties
	t.status.Runs[runID].Started = tcclient.Time(time.Now())
	t.status.Runs[runID].WorkerGroup = payload.WorkerGroup
	t.status.Runs[runID].WorkerID = payload.WorkerID
	t.status.Runs[runID].TakenUntil = tcclient.Time(time.Now().Add(5 * time.Minute))
	t.status.Runs[runID].State = statusRunning
	t.status.State = statusRunning

	return tcqueue.TaskClaimResponse{
		Credentials: fakeCredentials,
		RunID:       int64(runID),
		Status:      t.status,
		TakenUntil:  t.status.Runs[runID].TakenUntil,
		Task:        t.task,
		WorkerGroup: payload.WorkerGroup,
		WorkerID:    payload.WorkerID,
	}
}

func (q *FakeQueue) reclaimTask(taskID string, runID int) interface{} {
	t, ok := q.tasks[taskID]
	if !ok || len(t.status.Runs) <= runID {
		return resourceNotFoundError
	}

	// If run is not running we have a conflict
	if t.status.Runs[runID].State != statusRunning {
		return restError{
			StatusCode: http.StatusConflict,
			Code:       "RequestConflict",
			Message:    "run not running",
		}
	}

	// Update takenUntil
	t.status.Runs[runID].TakenUntil = tcclient.Time(time.Now().Add(5 * time.Minute))

	return tcqueue.TaskReclaimResponse{
		Credentials: fakeCredentials,
		RunID:       int64(runID),
		Status:      t.status,
		TakenUntil:  t.status.Runs[runID].TakenUntil,
		WorkerGroup: t.status.Runs[runID].WorkerGroup,
		WorkerID:    t.status.Runs[runID].WorkerID,
	}
}

func (q *FakeQueue) reportCompleted(taskID string, runID int) interface{} {
	t, ok := q.tasks[taskID]
	if !ok || len(t.status.Runs) <= runID {
		return resourceNotFoundError
	}

	// If completed we do nothing, idempontent requests allowed
	if t.status.Runs[runID].State == statusCompleted {
		return tcqueue.TaskStatusResponse{Status: t.status}
	}

	// if not running we have a conflict
	if t.status.Runs[runID].State != statusRunning {
		return restError{
			StatusCode: http.StatusConflict,
			Code:       "RequestConflict",
			Message:    "run is already resolved or pending",
		}
	}

	// Resolve the run
	t.status.Runs[runID].State = statusCompleted
	t.status.Runs[runID].ReasonResolved = "completed"
	t.status.Runs[runID].Resolved = tcclient.Time(time.Now())
	t.status.State = statusCompleted

	return tcqueue.TaskStatusResponse{Status: t.status}
}

func (q *FakeQueue) reportFailed(taskID string, runID int) interface{} {
	t, ok := q.tasks[taskID]
	if !ok || len(t.status.Runs) <= runID {
		return resourceNotFoundError
	}

	// If failed we do nothing, idempontent requests allowed
	if t.status.Runs[runID].State == statusFailed {
		return tcqueue.TaskStatusResponse{Status: t.status}
	}

	// if not running we have a conflict
	if t.status.Runs[runID].State != statusRunning {
		return restError{
			StatusCode: http.StatusConflict,
			Code:       "RequestConflict",
			Message:    "run is already resolved or pending",
		}
	}

	// Resolve the run
	t.status.Runs[runID].State = statusFailed
	t.status.Runs[runID].ReasonResolved = "failed"
	t.status.Runs[runID].Resolved = tcclient.Time(time.Now())
	t.status.State = statusFailed

	return tcqueue.TaskStatusResponse{Status: t.status}
}

func (q *FakeQueue) reportException(taskID string, runID int, payload *tcqueue.TaskExceptionRequest) interface{} {
	t, ok := q.tasks[taskID]
	if !ok || len(t.status.Runs) <= runID {
		return resourceNotFoundError
	}

	// If completed we do nothing, idempontent requests allowed
	if t.status.Runs[runID].State == statusException &&
		t.status.Runs[runID].ReasonResolved == payload.Reason {
		return tcqueue.TaskStatusResponse{Status: t.status}
	}

	// if not running we have a conflict
	if t.status.Runs[runID].State != statusRunning {
		return restError{
			StatusCode: http.StatusConflict,
			Code:       "RequestConflict",
			Message:    "run is already resolved or pending",
		}
	}

	// Resolve the run
	t.status.Runs[runID].State = statusException
	t.status.Runs[runID].ReasonResolved = payload.Reason
	t.status.Runs[runID].Resolved = tcclient.Time(time.Now())
	t.status.State = statusException

	// Add a retry run
	if t.status.RetriesLeft > 0 && (payload.Reason == "worker-shutdown" || payload.Reason == "intermittent-task") {
		t.status.RetriesLeft--
		reason := "retry"
		if payload.Reason == "intermittent-task" {
			reason = "task-retry"
		}
		t.status.Runs = append(t.status.Runs, run{
			ReasonCreated: reason,
			RunID:         int64(len(t.status.Runs)),
			Scheduled:     tcclient.Time(time.Now()),
			State:         statusPending,
		})
		t.artifacts = append(t.artifacts, make(map[string]artifact))
		t.status.State = statusPending
	}

	return tcqueue.TaskStatusResponse{Status: t.status}
}

func (q *FakeQueue) createArtifact(taskID string, runID int, name string, payload []byte, r *http.Request) interface{} {
	// Parse payload
	var a artifact
	if err := json.Unmarshal(payload, &a); err != nil {
		return invalidJSONPayloadError
	}

	// Find task
	t, ok := q.tasks[taskID]
	if !ok || len(t.status.Runs) <= runID {
		return resourceNotFoundError
	}

	// if artifact already exists, and it doesn't match and it's not a reference
	if a2, ok := t.artifacts[runID][name]; ok && !isJSONEqual(a, a2) &&
		!(a.StorageType == storageTypeReference && a2.StorageType == storageTypeReference) {
		return restError{
			StatusCode: http.StatusConflict,
			Code:       "RequestConflict",
			Message:    "Artifact already exists with different definition",
		}
	}

	// Recover data in case there was some stored
	a.Data = t.artifacts[runID][name].Data
	t.artifacts[runID][name] = a

	var result struct {
		StorageType string    `json:"storageType"`
		Expires     time.Time `json:"expires,omitempty"`
		ContentType string    `json:"contentType,omitempty"`
		PutURL      string    `json:"putUrl,omitempty"`
	}
	result.StorageType = a.StorageType

	// Set PutURL if s3 or azure
	if a.StorageType == storageTypeS3 || a.StorageType == storageTypeAzure {
		proto := "http"
		if r.TLS != nil {
			proto = "https"
		}
		result.PutURL = fmt.Sprintf(
			"%s://%s/internal/task/%s/runs/%d/put-artifact/%s",
			proto, r.Host, taskID, runID, name,
		)
		result.Expires = time.Now().Add(5 * time.Minute)
	}

	return result
}

func (q *FakeQueue) internalPutArtifact(taskID string, runID int, name string, payload []byte, r *http.Request) interface{} {
	// Find task
	t, ok := q.tasks[taskID]
	if !ok || len(t.status.Runs) <= runID {
		return resourceNotFoundError
	}

	// check that the artifact was created first
	if a, ok := t.artifacts[runID][name]; !ok || (a.StorageType != storageTypeS3 && a.StorageType != storageTypeAzure) {
		return restError{
			StatusCode: http.StatusForbidden,
			Code:       "InvalidPutURL",
			Message:    "This URL was not returned from createArtifact",
		}
	}

	// Store payload
	a := t.artifacts[runID][name]
	a.Data = payload
	a.ContentEncoding = r.Header.Get("Content-Encoding")
	t.artifacts[runID][name] = a

	return map[string]interface{}{
		"upload": "OK",
	}
}

func (q *FakeQueue) getArtifact(taskID string, runID int, name string) interface{} {
	// Find task
	t, ok := q.tasks[taskID]
	if !ok || len(t.status.Runs) <= runID {
		return resourceNotFoundError
	}

	if a, ok := t.artifacts[runID][name]; ok {
		switch a.StorageType {
		case storageTypeS3, storageTypeAzure:
			return rawResponse{
				StatusCode:      http.StatusOK,
				ContentType:     a.ContentType,
				ContentEncoding: a.ContentEncoding,
				Payload:         a.Data,
			}
		case storageTypeReference:
			return redirectResponse{
				StatusCode: http.StatusSeeOther,
				Location:   a.URL,
			}
		case storageTypeError:
		default:
			panic(fmt.Sprintf("unknown artifact storageType: %s", a.StorageType))
		}
	}

	return restError{
		StatusCode: http.StatusNotFound,
		Code:       "ResourceNotFound",
		Message:    "No such artifact found",
	}
}

func (q *FakeQueue) getLatestArtifact(taskID, name string) interface{} {
	// Find task
	t, ok := q.tasks[taskID]
	if !ok || len(t.status.Runs) == 0 {
		return resourceNotFoundError
	}

	return q.getArtifact(taskID, len(t.status.Runs)-1, name)
}

func (q *FakeQueue) listArtifacts(taskID string, runID int) interface{} {
	// Find task
	t, ok := q.tasks[taskID]
	if !ok || len(t.status.Runs) <= runID {
		return resourceNotFoundError
	}

	var result tcqueue.ListArtifactsResponse
	for name, a := range t.artifacts[runID] {
		result.Artifacts = append(result.Artifacts, struct {
			ContentType string        `json:"contentType"`
			Expires     tcclient.Time `json:"expires"`
			Name        string        `json:"name"`
			StorageType string        `json:"storageType"`
		}{
			ContentType: a.ContentType,
			Expires:     tcclient.Time(a.Expires),
			Name:        name,
			StorageType: a.StorageType,
		})
	}
	return result
}

func (q *FakeQueue) listLatestArtifacts(taskID string) interface{} {
	// Find task
	t, ok := q.tasks[taskID]
	if !ok || len(t.status.Runs) == 0 {
		return resourceNotFoundError
	}

	return q.listArtifacts(taskID, len(t.status.Runs)-1)
}

func (q *FakeQueue) pendingTasks(provisionerID, workerType string) interface{} {
	count := 0
	for _, t := range q.tasks {
		if t.status.State == statusPending && t.status.ProvisionerID == provisionerID && t.status.WorkerType == workerType {
			count++
		}
	}

	return tcqueue.CountPendingTasksResponse{
		PendingTasks:  int64(count),
		ProvisionerID: provisionerID,
		WorkerType:    workerType,
	}
}

func (q *FakeQueue) ping() interface{} {
	return map[string]interface{}{
		"status": "running",
	}
}

func (q *FakeQueue) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	debug("%s %s", r.Method, r.URL.Path)

	// Read body before we lock
	var data []byte
	if r.Body != nil {
		defer r.Body.Close()
		var rerr error
		data, rerr = ioutil.ReadAll(r.Body)
		if rerr != nil {
			debug("failed to read REST payload, error: %s", rerr)
			reply(w, r, restError{
				StatusCode: http.StatusInternalServerError,
				Code:       "PayloadTransmissionError",
				Message:    "Some error reading the payload",
			})
			return
		}
	}

	// Always initialize and lock
	q.initAndLock()
	defer q.m.Unlock()

	// Always notify other threads that state may have changed
	defer q.c.Broadcast()

	// Always reconcileTasks
	reconcileTasks(q.tasks)
	defer reconcileTasks(q.tasks)

	// handle URL path
	p := r.URL.Path

	// GET  /task/<taskId>
	if m := patternTask.FindStringSubmatch(p); len(m) > 0 && r.Method == http.MethodGet {
		debug(" -> queue.task(%s)", m[1])
		reply(w, r, q.task(m[1]))
		return
	}

	// PUT  /task/<taskId>
	if m := patternCreateTask.FindStringSubmatch(p); len(m) > 0 && r.Method == http.MethodPut {
		debug(" -> queue.createTask(%s, ...)", m[1])
		var payload tcqueue.TaskDefinitionRequest
		if err := json.Unmarshal(data, &payload); err != nil {
			reply(w, r, invalidJSONPayloadError)
			return
		}
		reply(w, r, q.createTask(m[1], &payload))
		return
	}

	// GET  /task/<taskId>/status
	if m := patternStatus.FindStringSubmatch(p); len(m) > 0 && r.Method == http.MethodGet {
		debug(" -> queue.status(%s)", m[1])
		reply(w, r, q.status(m[1]))
		return
	}

	// POST /task/<taskId>/cancel
	if m := patternCancelTask.FindStringSubmatch(p); len(m) > 0 && r.Method == http.MethodPost {
		debug(" -> queue.cancelTask(%s)", m[1])
		reply(w, r, q.cancelTask(m[1]))
		return
	}

	// POST /task/<taskId>/runs/<runId>/claim
	if m := patternClaimTask.FindStringSubmatch(p); len(m) > 0 && r.Method == http.MethodPost {
		runID, _ := strconv.Atoi(m[2])
		debug(" -> queue.claimTask(%s, %d)", m[1], runID)
		var payload tcqueue.TaskClaimRequest
		if err := json.Unmarshal(data, &payload); err != nil {
			reply(w, r, invalidJSONPayloadError)
			return
		}
		reply(w, r, q.claimTask(m[1], runID, &payload))
		return
	}

	// POST /task/<taskId>/runs/<runId>/reclaim
	if m := patternReclaimTask.FindStringSubmatch(p); len(m) > 0 && r.Method == http.MethodPost {
		runID, _ := strconv.Atoi(m[2])
		debug(" -> queue.reclaimTask(%s, %d)", m[1], runID)
		reply(w, r, q.reclaimTask(m[1], runID))
		return
	}

	// POST /task/<taskId>/runs/<runId>/completed
	if m := patternReportCompleted.FindStringSubmatch(p); len(m) > 0 && r.Method == http.MethodPost {
		runID, _ := strconv.Atoi(m[2])
		debug(" -> queue.reportCompleted(%s, %d)", m[1], runID)
		reply(w, r, q.reportCompleted(m[1], runID))
		return
	}

	// POST /task/<taskId>/runs/<runId>/failed
	if m := patternReportFailed.FindStringSubmatch(p); len(m) > 0 && r.Method == http.MethodPost {
		runID, _ := strconv.Atoi(m[2])
		debug(" -> queue.reportFailed(%s, %d)", m[1], runID)
		reply(w, r, q.reportFailed(m[1], runID))
		return
	}

	// POST /task/<taskId>/runs/<runId>/exception
	if m := patternReportException.FindStringSubmatch(p); len(m) > 0 && r.Method == http.MethodPost {
		runID, _ := strconv.Atoi(m[2])
		var payload tcqueue.TaskExceptionRequest
		if err := json.Unmarshal(data, &payload); err != nil {
			reply(w, r, invalidJSONPayloadError)
			return
		}
		debug(" -> queue.reportException(%s, %d, {reason: %s})", m[1], runID, payload.Reason)
		reply(w, r, q.reportException(m[1], runID, &payload))
		return
	}

	// POST /task/<taskId>/runs/<runId>/artifacts/<name>
	if m := patternCreateArtifact.FindStringSubmatch(p); len(m) > 0 && r.Method == http.MethodPost {
		runID, _ := strconv.Atoi(m[2])
		debug(" -> queue.createArtifact(%s, %d, %s, ...)", m[1], runID, m[3])
		reply(w, r, q.createArtifact(m[1], runID, m[3], data, r))
		return
	}

	// PUT  /internal/task/<taskId>/runs/<runId>/artifacts/<name>
	if m := patternArtifactPutURL.FindStringSubmatch(p); len(m) > 0 && r.Method == http.MethodPut {
		debug(" -> s3/azure")
		runID, _ := strconv.Atoi(m[2])
		reply(w, r, q.internalPutArtifact(m[1], runID, m[3], data, r))
		return
	}

	// GET  /task/<taskId>/runs/<runId>/artifacts/<name>
	if m := patternGetArtifact.FindStringSubmatch(p); len(m) > 0 && r.Method == http.MethodGet {
		runID, _ := strconv.Atoi(m[2])
		debug(" -> queue.getArtifact(%s, %d, %s)", m[1], runID, m[3])
		reply(w, r, q.getArtifact(m[1], runID, m[3]))
		return
	}

	// GET  /task/<taskId>/runs/<runId>/artifacts
	if m := patternListArtifacts.FindStringSubmatch(p); len(m) > 0 && r.Method == http.MethodGet {
		runID, _ := strconv.Atoi(m[2])
		debug(" -> queue.listArtifacts(%s, %d)", m[1], runID)
		reply(w, r, q.listArtifacts(m[1], runID))
		return
	}

	// GET  /task/<taskId>/artifacts/<name>
	if m := patternGetLatestArtifact.FindStringSubmatch(p); len(m) > 0 && r.Method == http.MethodGet {
		debug(" -> queue.getLatestArtifact(%s, %s)", m[1], m[2])
		reply(w, r, q.getLatestArtifact(m[1], m[2]))
		return
	}

	// GET  /task/<taskId>/artifacts
	if m := patternListLatestArtifacts.FindStringSubmatch(p); len(m) > 0 && r.Method == http.MethodGet {
		debug(" -> queue.listLatestArtifacts(%s)", m[1])
		reply(w, r, q.listLatestArtifacts(m[1]))
		return
	}

	// POST /claim-work/<provisionerId>/<workerType>
	if m := patternClaimWork.FindStringSubmatch(p); len(m) > 0 && r.Method == http.MethodPost {
		debug(" -> queue.claimWork(%s, %s, ...)", m[1], m[2])
		var payload tcqueue.ClaimWorkRequest
		if err := json.Unmarshal(data, &payload); err != nil {
			reply(w, r, invalidJSONPayloadError)
			return
		}
		reply(w, r, q.claimWork(m[1], m[2], &payload, w))
		return
	}

	// GET  /pending/<provisionerId>/<workerType>
	if m := patternPendingTasks.FindStringSubmatch(p); len(m) > 0 && r.Method == http.MethodGet {
		debug(" -> queue.pendingTasks(%s, %s)", m[1], m[2])
		reply(w, r, q.pendingTasks(m[1], m[2]))
		return
	}

	// GET  /ping
	if patternPing.MatchString(p) && r.Method == http.MethodGet {
		debug(" -> queue.ping()")
		reply(w, r, q.ping())
		return
	}

	// Reply with generic error
	reply(w, r, restError{
		StatusCode: http.StatusNotFound,
		Code:       "UnknownEndPoint",
		Message:    "Invalid request URL or method. No matching end-point found!",
	})
}
