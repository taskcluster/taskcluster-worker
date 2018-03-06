package workertest

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/taskcluster/slugid-go/slugid"
	tcclient "github.com/taskcluster/taskcluster-client-go"
	"github.com/taskcluster/taskcluster-client-go/queue"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
	"github.com/taskcluster/taskcluster-worker/worker"
	"github.com/taskcluster/taskcluster-worker/worker/workertest/fakequeue"
)

const defaultTestCaseTimeout = 10 * time.Minute

// Case is a worker test case
type Case struct {
	Engine            string        // Engine to be used
	EngineConfig      string        // Engine configuration as JSON
	PluginConfig      string        // Configuration of plugins, see plugins.PluginManagerConfigSchema()
	Tasks             []Task        // Tasks to create and associated assertions
	Concurrency       int           // Worker concurrency, if zero defaulted to 1 and tasks will sequantially dependent
	StoppedGracefully bool          // True, if worker is expected to stop gracefully
	StoppedNow        bool          // True, if worker is expected to stop now
	Timeout           time.Duration // Test timeout, defaults to 10 Minute
	EnableSuperseding bool          // Enable superseding in the worker
}

// A Task to be included in a worker test case
type Task struct {
	TaskID          string                  // Optional taskID (use slugid.Nice())
	Title           string                  // Optional title (for debugging)
	Scopes          []string                // Task scopes
	Payload         string                  // Task payload as JSON
	IgnoreState     bool                    // Ignore Success and Exception
	Success         bool                    // True, if task should be successfully
	Exception       runtime.ExceptionReason // Reason, if exception is expected
	Artifacts       ArtifactAssertions      // Mapping from artifact name to assertion
	AllowAdditional bool                    // True, if additional artifacts is allowed
	Status          StatusAssertion         // Optional, custom assertion on status and queue
}

// A StatusAssertion is a function that can make an assertion on a task status
type StatusAssertion func(t *testing.T, q *queue.Queue, status queue.TaskStatusStructure)

// An ArtifactAssertions is a mapping from artifact name to assertion for the
// artifact. If mapping to nil value, any artifact will be permitted.
type ArtifactAssertions map[string]func(t *testing.T, a Artifact)

// Artifact contains artifact meta-data.
type Artifact struct {
	ContentType     string
	Expires         time.Time
	Name            string
	StorageType     string
	Data            []byte
	ContentEncoding string
}

// provisionerId/workerType for test cases, access granted by role:
//   assume:project:taskcluster:worker-test-scopes
var dummyProvisionerID = "test-dummy-provisioner"

func dummyWorkerType() string {
	return "dummy-worker-" + slugid.V4()[:9]
}

// TestWithFakeQueue runs integration tests against FakeQueue
func (c Case) TestWithFakeQueue(t *testing.T) {
	// Create FakeQueue
	fq := fakequeue.New()
	s := httptest.NewServer(fq)
	defer s.Close()

	// Create listener
	l := fakequeue.NewFakeQueueListener(fq)

	// Create queue client
	q := queue.New(&tcclient.Credentials{
		// Long enough to pass schema validation
		ClientID:    "dummy-test-client-id",
		AccessToken: "non-secret-dummy-test-access-token",
	})
	q.BaseURL = s.URL

	c.testWithQueue(t, q, l)
}

// TestWithRealQueue runs integration tests against production queue
func (c Case) TestWithRealQueue(t *testing.T) {
	u := os.Getenv("PULSE_USERNAME")
	p := os.Getenv("PULSE_PASSWORD")
	if u == "" || p == "" {
		t.Skip("Skipping integration tests, because PULSE_USERNAME and PULSE_PASSWORD are not specified")
	}

	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	// Create listener
	l, err := fakequeue.NewPulseListener(u, p)
	require.NoError(t, err, "Failed to create PulseListener")

	// Create queue client
	q := queue.New(&tcclient.Credentials{
		ClientID:    os.Getenv("TASKCLUSTER_CLIENT_ID"),
		AccessToken: os.Getenv("TASKCLUSTER_ACCESS_TOKEN"),
		Certificate: os.Getenv("TASKCLUSTER_CERTIFICATE"),
	})
	if os.Getenv("QUEUE_BASE_URL") != "" {
		q.BaseURL = os.Getenv("QUEUE_BASE_URL")
	}

	c.testWithQueue(t, q, l)
}

// Test runs the test case
func (c Case) Test(t *testing.T) {
	passedFake := t.Run("FakeQueue", c.TestWithFakeQueue)
	// We don't run real integration tests if the FakeQueue tests fails
	// This is aimed to avoid polluting the queue and reducing feedback cycle.
	// You can manually call TestWithRealQueue(t), if you want to debug it.
	if passedFake {
		t.Run("RealQueue", c.TestWithRealQueue)
	} else {
		t.Run("RealQueue", func(t *testing.T) {
			t.Skip("Skipping integration tests, because FakeQueue tests failed")
		})
	}
}

func mustUnmarshalJSON(data string) interface{} {
	var v interface{}
	err := json.Unmarshal([]byte(data), &v)
	if err != nil {
		panic(fmt.Sprintf("Failed to parse JSON, error: %s, json: '%s'", err, data))
	}
	return v
}

func (c Case) testWithQueue(t *testing.T, q *queue.Queue, l fakequeue.Listener) {
	// Create config
	tempFolder := path.Join(os.TempDir(), slugid.Nice())
	defer os.RemoveAll(tempFolder)
	concurrency := c.Concurrency
	if concurrency == 0 {
		concurrency = 1
	}
	creds := map[string]interface{}{
		"clientId":    q.Credentials.ClientID,
		"accessToken": q.Credentials.AccessToken,
	}
	if q.Credentials.Certificate != "" {
		creds["certificate"] = q.Credentials.Certificate
	}
	workerID := "dummy-worker-" + slugid.V4()[:9]
	workerType := dummyWorkerType()
	config := map[string]interface{}{
		"credentials": creds,
		"engine":      c.Engine,
		"engines": map[string]interface{}{
			c.Engine: mustUnmarshalJSON(c.EngineConfig),
		},
		"minimumDiskSpace": 0,
		"minimumMemory":    0,
		"monitor":          mustUnmarshalJSON(`{"panicOnError": false, "type": "mock"}`),
		"plugins":          mustUnmarshalJSON(c.PluginConfig),
		"queueBaseUrl":     q.BaseURL,
		"temporaryFolder":  tempFolder,
		"webHookServer":    mustUnmarshalJSON(`{"provider": "localhost"}`),
		"worker": map[string]interface{}{
			"concurrency":         concurrency,
			"minimumReclaimDelay": 30,
			"pollingInterval":     1,
			"reclaimOffset":       30,
			"workerGroup":         "test-dummy-workers",
			"workerId":            workerID,
			"provisionerId":       dummyProvisionerID,
			"workerType":          workerType,
			"enableSuperseding":   c.EnableSuperseding,
		},
	}
	err := worker.ConfigSchema().Validate(config)
	require.NoError(t, err, "Failed to validate worker config against schema")

	// Create worker
	w, err := worker.New(config)
	require.NoError(t, err, "Failed to create worker")

	// Create taskIDs
	taskIDs := make([]string, len(c.Tasks))
	for i := range taskIDs {
		if c.Tasks[i].TaskID != "" {
			taskIDs[i] = c.Tasks[i].TaskID
		} else {
			taskIDs[i] = slugid.Nice()
		}
	}

	// Setup event listeners
	events := make([]<-chan error, len(c.Tasks))
	util.Spawn(len(c.Tasks), func(i int) {
		events[i] = l.WaitForTask(taskIDs[i])
	})

	// Wait for tasks to be resolved
	var tasksResolved atomics.Once
	go tasksResolved.Do(func() {
		// Wait for events
		debug("Waiting for tasks to be resolved")
		util.Spawn(len(c.Tasks), func(i int) {
			err := <-events[i]
			assert.NoError(t, err, "Failed to listen for task %d", i)
			debug("Finished waiting for %s", taskIDs[i])
			if err != nil {
				debug("Error '%s' waiting for %s", taskIDs[i], err)
			}
		})
	})

	// Create tasks
	for i, task := range c.Tasks {
		tdef := queue.TaskDefinitionRequest{
			ProvisionerID: dummyProvisionerID,
			WorkerType:    workerType,
			Created:       tcclient.Time(time.Now()),
			Deadline:      tcclient.Time(time.Now().Add(60 * time.Minute)),
			Expires:       tcclient.Time(time.Now().Add(31 * 24 * 60 * time.Minute)),
			Payload:       json.RawMessage(task.Payload),
		}
		// If tasks are to run sequantially, we'll make them dependent
		if c.Concurrency == 0 && i > 0 {
			tdef.Dependencies = []string{taskIDs[i-1]}
			tdef.Requires = "all-resolved"
		}
		title := task.Title
		if title == "" {
			title = fmt.Sprintf("Task %d", i)
		}
		tdef.Scopes = task.Scopes
		tdef.Metadata.Name = title
		tdef.Metadata.Description = "Task from taskcluster-worker integration tests"
		tdef.Metadata.Source = "https://github.com/taskcluster/taskcluster-worker/tree/master/worker/workertest/workertest.go"
		tdef.Metadata.Owner = "jonasfj@mozilla.com"
		debug("creating task '%s' as taskId: %s", title, taskIDs[i])
		_, err := q.CreateTask(taskIDs[i], &tdef)
		require.NoError(t, err, "Failed to create task: %s", title)
	}

	// Start worker
	var serr error
	var stopped atomics.Once
	go stopped.Do(func() {
		debug("starting worker with workerType: %s and workerID: %s", workerType, workerID)
		serr = w.Start()
		debug("worker stopped")
	})

	// Wait for events to have been handled
	timeout := c.Timeout
	if timeout == 0 {
		timeout = defaultTestCaseTimeout
	}
	select {
	case <-tasksResolved.Done():
	case <-time.After(timeout):
		assert.Fail(t, "Test case timed out, see workertest.Case.Timeout Property!")
		debug("worker.StopNow() because of test case timeout")
		w.StopNow()
		// We give it 30s to stop now, otherwise we end the test-case
		select {
		case <-stopped.Done():
		case <-time.After(30 * time.Second):
			debug("worker.StopNow() didn't stop after 30s")
		}
		return
	}

	// if we expect the worker to stop then we don't want to stop it here
	if !c.StoppedGracefully && !c.StoppedNow {
		// Stop worker
		debug("gracefully stopping worker (since test-case isn't stopping the worker)")
		w.StopGracefully()
	}

	// Wait for the worker to stop
	select {
	case <-stopped.Done():
	case <-time.After(30 * time.Second):
		assert.Fail(t, "Expected worker to stop")
	}

	// Verify assertions
	// We must do this after the worker has stopped, since tasks resolved
	// with exception can have artifacts added after resolution.
	debug("Verifying task assertions")
	// We could run these in parallel, but debugging is easier if we don't...
	for i, task := range c.Tasks {
		title := task.Title
		if title == "" {
			title = fmt.Sprintf("Task %d", i)
		}
		t.Run(title, func(t *testing.T) {
			verifyAssertions(t, title, taskIDs[i], task, q)
		})
	}

	// Check the stopping condition
	if c.StoppedNow {
		assert.Exactly(t, worker.ErrWorkerStoppedNow, serr, "Expected StoppedNow!")
	} else {
		assert.NoError(t, serr, "Expected worker to stop gracefully")
	}
}
