package worker

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/httpbackoff"
	tcclient "github.com/taskcluster/taskcluster-client-go"
	"github.com/taskcluster/taskcluster-client-go/auth"
	"github.com/taskcluster/taskcluster-client-go/queue"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
	"github.com/taskcluster/taskcluster-worker/runtime/gc"
	"github.com/taskcluster/taskcluster-worker/runtime/monitoring"
	"github.com/taskcluster/taskcluster-worker/runtime/webhookserver"
	"github.com/taskcluster/taskcluster-worker/worker/taskrun"
)

// A Worker processes tasks
type Worker struct {
	// New
	garbageCollector *gc.GarbageCollector
	temporaryStorage runtime.TemporaryFolder
	environment      runtime.Environment
	lifeCycleTracker runtime.LifeCycleTracker
	webhookserver    webhookserver.Server
	engine           engines.Engine
	plugin           plugins.Plugin
	queue            client.Queue
	queueBaseURL     string
	options          options
	monitor          runtime.Monitor
	// State
	started     atomics.Barrier
	activeTasks taskCounter
}

// New creates a new Worker
func New(config interface{}) (w *Worker, err error) {
	var c configType
	schematypes.MustValidateAndMap(ConfigSchema(), config, &c)

	// Create monitor
	a := auth.New(&c.Credentials)
	if c.AuthBaseURL != "" {
		a.BaseURL = c.AuthBaseURL
	}
	monitor := monitoring.New(c.Monitor, a)

	// Create worker
	w = &Worker{
		monitor:          monitor.WithPrefix("worker"),
		garbageCollector: gc.New(c.TemporaryFolder, c.MinimumDiskSpace, c.MinimumMemory),
		queueBaseURL:     c.QueueBaseURL,
		options:          c.WorkerOptions,
	}

	// Create queue client
	w.queue = w.newQueueClient(c.Credentials)

	// Create temporary storage
	w.temporaryStorage, err = runtime.NewTemporaryStorage(c.TemporaryFolder)
	if err != nil {
		w.monitor.ReportError(err, "worker.New() failed to create TemporaryStorage")
		err = runtime.ErrFatalInternalError
		return
	}

	// Create webhookserver
	w.webhookserver, err = webhookserver.NewServer(c.WebHookServer)
	if err != nil {
		w.monitor.ReportError(err, "worker.New() failed to setup webhookserver")
		err = runtime.ErrFatalInternalError
		return
	}

	// Create environment
	w.environment = runtime.Environment{
		Monitor:          monitor,
		GarbageCollector: w.garbageCollector,
		TemporaryStorage: w.temporaryStorage,
		WebHookServer:    w.webhookserver,
		Worker:           &w.lifeCycleTracker,
	}

	// Create engine
	provider := engines.Engines()[c.Engine]
	if _, ok := c.EngineConfig[c.Engine]; !ok {
		return nil, fmt.Errorf("missing engine config for '%s'", c.Engine)
	}
	w.engine, err = provider.NewEngine(engines.EngineOptions{
		Environment: &w.environment,
		Monitor:     monitor.WithPrefix("engine").WithTag("engine", c.Engine),
		Config:      c.EngineConfig[c.Engine],
	})
	if err != nil {
		w.monitor.ReportError(err, "worker.New() failed to create engine")
		err = runtime.ErrFatalInternalError
		return
	}

	// Create plugin
	w.plugin, err = plugins.NewPluginManager(plugins.PluginOptions{
		Environment: &w.environment,
		Engine:      w.engine,
		Monitor:     monitor.WithPrefix("plugin"),
		Config:      c.Plugins,
	})
	if err != nil {
		w.monitor.ReportError(err, "worker.New() failed to create plugin")
		err = runtime.ErrFatalInternalError
		return
	}

	// Check payload schema conflicts
	_, err = schematypes.Merge(
		w.engine.PayloadSchema(),
		w.plugin.PayloadSchema(),
	)
	if err != nil {
		w.monitor.ReportError(err, "worker.New() detected payload schema conflict between engine and plugin")
		err = runtime.ErrFatalInternalError
		return
	}

	return
}

// PayloadSchema returns the schema for task.payload
func (w *Worker) PayloadSchema() schematypes.Schema {
	payloadSchema, err := schematypes.Merge(
		w.engine.PayloadSchema(),
		w.plugin.PayloadSchema(),
	)
	if err != nil {
		// this should never happen, we try to do the above in New()
		panic(fmt.Sprintf(
			"Conflicting plugin and engine payload properties, error: %s", err,
		))
	}
	return payloadSchema
}

// ErrWorkerStoppedNow is used to communicate that the worker was forcefully
// stopped. This could also be triggered by a plugin or engine.
var ErrWorkerStoppedNow = errors.New("worker was interrupted by StopNow")

// Start process tasks, returns ErrWorkerStoppedNow if not stopped gracefully.
func (w *Worker) Start() error {
	// Ensure that we don't start running twice
	if !w.started.Fall() {
		panic("Worker.Start() cannot be called twice, worker cannot restart")
	}

	for !w.lifeCycleTracker.StoppingGracefully.IsFallen() {
		// Claim tasks
		N := w.options.Concurrency - w.activeTasks.Value()
		debug("queue.claimWork(%s, %s) with capacity: %d", w.options.ProvisionerID, w.options.WorkerType, N)
		claims, err := w.queue.ClaimWork(w.options.ProvisionerID, w.options.WorkerType, &queue.ClaimWorkRequest{
			WorkerGroup: w.options.WorkerGroup,
			WorkerID:    w.options.WorkerID,
			Tasks:       N,
		})
		if err != nil {
			w.monitor.ReportError(err, "failed to ClaimWork")
			w.plugin.ReportNonFatalError()
		}

		// If we have claims we MUST always handle, even if we have stopNow!
		if claims != nil {
			for _, claim := range claims.Tasks {
				// Start processing tasks
				debug("starting to process: %s/%d", claim.Status.TaskID, claim.RunID)
				w.activeTasks.Increment()
				go w.processClaim(claim)
			}
		}

		// If we received zero claims or encountered an error, we wait at-least
		// pollingInterval before polling again. We start the timer here, so it's
		// counting while we wait for capacity to be available.
		var delay <-chan time.Time
		if claims == nil || len(claims.Tasks) == 0 {
			delay = time.After(time.Duration(w.options.PollingInterval) * time.Second)
		} else {
			// If we received a task from the claimWork request then we don't have to
			// sleep before polling again. But we do have to wait for activeTasks to
			// drop below maximum allowed concurrency.
			delay = time.After(0)
		}

		// Wait for capacity to be available (delay is ticking while this happens)
		debug("waiting for activeTasks: %d < concurrency: %d", w.activeTasks.Value(), w.options.Concurrency)
		w.activeTasks.WaitForLessThan(w.options.Concurrency)

		// Wait for delay or stopGracefully
		debug("sleep before reclaiming, unless stopping gracefully")
		select {
		case <-delay:
		case <-w.lifeCycleTracker.StoppingGracefully.Barrier():
		}

		// Report idle time to plugins (so they can manage life-cycle)
		idle := w.activeTasks.IdleTime()
		if idle != 0 {
			w.plugin.ReportIdle(idle)
		}
	}

	// Wait for tasks to be done, or stopNow happens
	debug("waiting for active tasks to be resolved")
	w.activeTasks.WaitForIdle()

	// free resources when done running
	w.dispose()

	// Return ErrWorkerStoppedNow if the worker was forcefully stopped
	if w.lifeCycleTracker.StoppingNow.IsFallen() {
		return ErrWorkerStoppedNow
	}
	return nil
}

// anonymous struct from queue.ClaimWorkResponse.Tasks
type taskClaim struct {
	Credentials struct {
		AccessToken string `json:"accessToken"`
		Certificate string `json:"certificate"`
		ClientID    string `json:"clientId"`
	} `json:"credentials"`
	RunID       int                          `json:"runId"`
	Status      queue.TaskStatusStructure    `json:"status"`
	TakenUntil  tcclient.Time                `json:"takenUntil"`
	Task        queue.TaskDefinitionResponse `json:"task"`
	WorkerGroup string                       `json:"workerGroup"`
	WorkerID    string                       `json:"workerId"`
}

// Utility function to create a queue client object
func (w *Worker) newQueueClient(creds tcclient.Credentials) client.Queue {
	q := queue.New(&creds)
	if w.queueBaseURL != "" {
		q.BaseURL = w.queueBaseURL
	}
	return q
}

// reclaimDelay returns the delay before reclaiming given takenUntil
func (w *Worker) reclaimDelay(takenUntil time.Time) time.Duration {
	delay := time.Until(takenUntil) - time.Duration(w.options.ReclaimOffset)*time.Second
	// Never delay less than MinimumReclaimDelay
	if delay < time.Duration(w.options.MinimumReclaimDelay)*time.Second {
		return time.Duration(w.options.MinimumReclaimDelay) * time.Second
	}
	return delay
}

// processClaim is responsible for processing a task, reclaiming the task and
// aborting it with worker-shutdown with w.stopNow is unblocked, and decrements
// activeTasks when done
func (w *Worker) processClaim(claim taskClaim) {
	// Decrement number of active tasks when we're done processing the task
	defer w.activeTasks.Decrement()

	// Create monitor for this task
	monitor := w.monitor.WithTags(map[string]string{
		"taskId": claim.Status.TaskID,
		"runId":  strconv.Itoa(claim.RunID),
	})

	// Create task client
	q := w.newQueueClient(tcclient.Credentials{
		ClientID:    claim.Credentials.ClientID,
		AccessToken: claim.Credentials.AccessToken,
		Certificate: claim.Credentials.Certificate,
	})

	// Create a taskrun
	var payload map[string]interface{}
	if json.Unmarshal(claim.Task.Payload, &payload) != nil {
		panic("unable to parse payload as JSON, this shouldn't be possible")
	}
	run := taskrun.New(taskrun.Options{
		Environment: w.environment,
		Engine:      w.engine,
		Plugin:      w.plugin,
		Monitor:     monitor.WithPrefix("taskrun"),
		Queue:       q,
		Payload:     payload,
		TaskInfo: runtime.TaskInfo{
			TaskID:   claim.Status.TaskID,
			RunID:    claim.RunID,
			Created:  time.Time(claim.Task.Created),
			Deadline: time.Time(claim.Task.Deadline),
			Expires:  time.Time(claim.Task.Expires),
		},
	})

	// runId as string for use in requests
	runID := strconv.Itoa(claim.RunID)

	// Start reclaiming
	stopReclaiming := atomics.Barrier{}
	reclaimingDone := atomics.Barrier{}
	go func() {
		defer reclaimingDone.Fall()
		takenUntil := time.Time(claim.TakenUntil)
		for {
			// Wait for reclaim delay, stop of reclaiming, or stopNow called
			select {
			case <-stopReclaiming.Barrier():
				return
			case <-w.lifeCycleTracker.StoppingNow.Barrier():
				run.Abort(taskrun.WorkerShutdown)
				return
			case <-time.After(w.reclaimDelay(takenUntil)):
			}

			// Reclaim task
			debug("queue.reclaimTask(%s, %d)", claim.Status.TaskID, claim.RunID)
			result, err := q.ReclaimTask(claim.Status.TaskID, runID)
			if err != nil {
				if e, ok := err.(httpbackoff.BadHttpResponseCode); ok && e.HttpResponseCode == 409 {
					run.Abort(taskrun.TaskCanceled)
					return
				}
				monitor.ReportWarning(err, "failed to reclaim task")
				continue // Maybe we'll have more luck next time
			}

			// Update takenUntil and create a new queue client
			takenUntil = time.Time(result.TakenUntil)
			q = w.newQueueClient(tcclient.Credentials{
				ClientID:    result.Credentials.ClientID,
				AccessToken: result.Credentials.AccessToken,
				Certificate: result.Credentials.Certificate,
			})
			run.SetQueueClient(q) // update queue client on the run
		}
	}()

	// Wait for taskrun to finish
	success, exception, reason := run.WaitForResult()

	// Stop reclaiming
	stopReclaiming.Fall()

	// Wait for reclaiming to end (we can't use q while it may be updated)
	<-reclaimingDone.Barrier()

	// Report task resolution
	debug("reporting task %s/%d resolved", claim.Status.TaskID, claim.RunID)
	var err error
	if exception {
		if reason != runtime.ReasonCanceled {
			_, err = q.ReportException(claim.Status.TaskID, runID, &queue.TaskExceptionRequest{
				Reason: reason.String(),
			})
		}
	} else {
		if success {
			_, err = q.ReportCompleted(claim.Status.TaskID, runID)
		} else {
			_, err = q.ReportFailed(claim.Status.TaskID, runID)
		}
	}
	if err != nil {
		monitor.ReportError(err, "failed to report task resolution")
		w.plugin.ReportNonFatalError() // This is bad, but no need for it to be fatal
	}

	// Dispose all resources
	err = run.Dispose()
	if err == runtime.ErrNonFatalInternalError {
		// Count it, but otherwise ignore
		w.plugin.ReportNonFatalError()
	} else if err != nil {
		if err != runtime.ErrFatalInternalError {
			// This is now allowed, but let's be defensive here
			monitor.ReportError(err, "TaskRun.Dispose() returned unhandled error")
		}
		monitor.Error("fatal error from TaskRun.Dispose() stopping now")
		w.StopNow()
	}
}

// StopNow aborts current tasks resolving worker-shutdown, and causes Work()
// to return an error.
func (w *Worker) StopNow() {
	w.lifeCycleTracker.StopNow()
}

// StopGracefully stops claiming new tasks and returns nil from Work() when
// all currently running tasks are done.
func (w *Worker) StopGracefully() {
	w.lifeCycleTracker.StopGracefully()
}

// dispose all resources
func (w *Worker) dispose() {
	hasErr := false

	// Collect all garbage
	switch err := w.garbageCollector.CollectAll(); err {
	case runtime.ErrFatalInternalError, runtime.ErrNonFatalInternalError:
		hasErr = true
	case nil:
	default:
		w.monitor.ReportError(err, "error during final garbage collection")
		hasErr = true
	}

	// Dispose plugin
	switch err := w.plugin.Dispose(); err {
	case runtime.ErrFatalInternalError, runtime.ErrNonFatalInternalError:
		hasErr = true
	case nil:
	default:
		w.monitor.ReportError(err, "error while disposing plugin")
		hasErr = true
	}

	// Dispose engine
	switch err := w.engine.Dispose(); err {
	case runtime.ErrFatalInternalError, runtime.ErrNonFatalInternalError:
		hasErr = true
	case nil:
	default:
		w.monitor.ReportError(err, "error while disposing engine")
		hasErr = true
	}

	// Stop webhookserver
	w.webhookserver.Stop()

	// Remove temporary storage
	switch err := w.temporaryStorage.Remove(); err {
	case runtime.ErrFatalInternalError, runtime.ErrNonFatalInternalError:
		hasErr = true
	case nil:
	default:
		w.monitor.ReportError(err, "error while removing temporary storage")
		hasErr = true
	}

	if hasErr {
		w.lifeCycleTracker.StoppingNow.Fall()
	}
}
