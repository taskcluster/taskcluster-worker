package worker

import (
	"time"

	schematypes "github.com/taskcluster/go-schematypes"
	tcclient "github.com/taskcluster/taskcluster-client-go"
	"github.com/taskcluster/taskcluster-client-go/queue"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/lifecyclepolicy"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
	"github.com/taskcluster/taskcluster-worker/runtime/webhookserver"
)

// A Worker processes tasks
type Worker struct {
	environment           runtime.Environment
	server                webhookserver.Server
	monitor               runtime.Monitor
	engine                engines.Engine
	plugin                plugins.Plugin
	queue                 client.Queue
	lifeCyclePolicyConfig interface{}
	provisionerID         string
	workerType            string
	workerGroup           string
	workerID              string
	pollingInterval       time.Duration
	reclaimOffset         time.Duration
	concurrency           int
	// State
	running        atomics.Bool
	activeTasks    atomics.Counter
	stopNow        atomics.Barrier
	stopGracefully atomics.Barrier
}

// New creates a new Worker
func New(options Options) (*Worker, error) {
	w := &Worker{}

	return w, nil
}

// PayloadSchema returns the schema for task.payload
func (w *Worker) PayloadSchema() schematypes.Schema {
	return nil
}

// Start process tasks, returns nil if stopped gracefully
func (w *Worker) Start() error {
	// Ensure that we don't start running twice
	if w.running.Swap(true) {
		panic("Worker.Start() called while worker is already running")
	}
	defer w.running.Set(false)

	// Reset barriers
	w.stopNow = atomics.Barrier{}
	w.stopGracefully = atomics.Barrier{}
	w.stopNow.Forward(w.stopGracefully.Fall) // stopNow implies stopGracefully
	defer w.stopNow.Fall()                   // Always lower the barrier

	// Create LifeCyclePolicy
	lifecycle := lifecyclepolicy.New(lifecyclepolicy.Options{
		Monitor: w.environment.Monitor.WithPrefix("life-cycle-policy"),
		Config:  w.lifeCyclePolicyConfig,
		Worker:  w,
	})

	// Track idle time
	idleTimer := atomics.StopWatch{}
	go func() {
		for {
			select {
			case <-w.activeTasks.Changed():
				if w.activeTasks.Value() > 0 {
					idleTimer.Reset() // Reset also stop the timer
				} else {
					idleTimer.Start()
				}
			case <-w.stopGracefully.Barrier():
				break
			}
		}
	}()

	for !w.stopGracefully.IsFallen() {
		// Claim tasks
		claims, err := w.queue.ClaimWork(w.provisionerID, w.workerType, &queue.ClaimWorkRequest{
			WorkerGroup: w.workerGroup,
			WorkerID:    w.workerID,
			Tasks:       w.concurrency - w.activeTasks.Value(),
		})
		if err != nil {
			w.monitor.ReportError(err, "failed to ClaimWork")
			lifecycle.ReportNonFatalError()
		}

		// If we have claims we MUST always handle, even if we have stopNow!
		if claims != nil {
			for _, claim := range claims.Tasks {
				// Start processing tasks
				w.activeTasks.Add(1)
				go w.processClaim(taskClaim{
					TaskID: claim.Status.TaskID,
					RunID:  claim.RunID,
					Task:   claim.Task,
					Credentials: tcclient.Credentials{
						ClientID:    claim.Credentials.ClientID,
						AccessToken: claim.Credentials.AccessToken,
						Certificate: claim.Credentials.Certificate,
					},
					TakenUntil: time.Time(claim.TakenUntil),
				}, lifecycle)
			}
			if len(claims.Tasks) > 0 {
				lifecycle.ReportTaskClaimed(len(claims.Tasks))
			}
		}

		// If we received zero claims or encountered an error, we wait at-least
		// pollingInterval before polling again. We start the timer here, so it's
		// counting while we wait for capacity to be available.
		var delay <-chan time.Time
		if claims == nil || len(claims.Tasks) == 0 {
			delay = time.After(w.pollingInterval)
		} else {
			delay = time.After(0)
		}

		// Wait for capacity to be available (delay is ticking while this happens)
		w.activeTasks.WaitForLessThan(w.concurrency)

		// Wait for delay or stopGracefully
		select {
		case <-delay:
		case <-w.stopGracefully.Barrier():
			break
		}

		idle := idleTimer.Elapsed()
		if idle != 0 {
			lifecycle.ReportIdle(idle)
		}
	}

	// Wait for tasks to be done, or stopNow happens
	w.activeTasks.WaitForZero()

	return nil
}

type taskClaim struct {
	TaskID      string
	RunID       int
	Task        queue.TaskDefinitionResponse
	Credentials tcclient.Credentials
	TakenUntil  time.Time
}

// processClaim is responsible for processing a task, reclaiming the task and
// aborting it with worker-shutdown with w.stopNow is unblocked, and decrements
// activeTasks when done
func (w *Worker) processClaim(claim taskClaim, lifecycle lifecyclepolicy.LifeCyclePolicy) {
	// Decrement number of active tasks when we're done processing the task
	defer w.activeTasks.Add(-1)
	// Measure resolution time and report it
	resolutionTimer := atomics.StopWatch{}
	resolutionTimer.Start()
	defer func() {
		lifecycle.ReportTaskResolved(resolutionTimer.Elapsed())
	}()

	go func() {
		select {
		case <-w.stopNow.Barrier():
			// Abort task with worker-shutdown
		}
	}()

	//TODO: Reclaim task
	//TODO: Create taskrun
}

// StopNow aborts current tasks resolving worker-shutdown, and causes Work()
// to return an error.
func (w *Worker) StopNow() {
	w.stopNow.Fall()
}

// StopGracefully stops claiming new tasks and returns nil from Work() when
// all currently running tasks are done.
func (w *Worker) StopGracefully() {
	w.stopGracefully.Fall()
}

// Dispose worker
func (w *Worker) Dispose() {
	if w.running.Get() { // Just a safety check
		panic("Worker.Dispose() called while worker is running")
	}
}
