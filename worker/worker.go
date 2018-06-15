package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/pkg/errors"
	got "github.com/taskcluster/go-got"
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/httpbackoff"
	tcclient "github.com/taskcluster/taskcluster-client-go"
	"github.com/taskcluster/taskcluster-client-go/tcauth"
	"github.com/taskcluster/taskcluster-client-go/tcqueue"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
	"github.com/taskcluster/taskcluster-worker/runtime/gc"
	"github.com/taskcluster/taskcluster-worker/runtime/monitoring"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
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
	plugin           *plugins.PluginManager
	queue            client.Queue
	queueBaseURL     string
	options          options
	monitor          runtime.Monitor
	// State
	started     atomics.Once
	activeTasks taskCounter
}

// New creates a new Worker
func New(config interface{}) (w *Worker, err error) {
	var c configType
	schematypes.MustValidateAndMap(ConfigSchema(), config, &c)

	if c.RootURL == "" {
		c.RootURL = "https://taskcluster.net"
	}

	rootURL, err := url.Parse(c.RootURL)
	if err != nil {
		return nil, err
	}

	// Create monitor
	a := tcauth.New(&c.Credentials)
	a.BaseURL = runtime.GetServiceURL(rootURL, "auth")
	monitor := monitoring.New(c.Monitor, a)

	// Create worker
	w = &Worker{
		monitor:          monitor.WithPrefix("worker"),
		garbageCollector: gc.New(c.TemporaryFolder, c.MinimumDiskSpace, c.MinimumMemory),
		queueBaseURL:     runtime.GetServiceURL(rootURL, "queue"),
		options:          c.WorkerOptions,
	}

	w.monitor.Info("starting up")

	// Create queue client that is aborted when life-cycle ends
	w.queue = w.newQueueClient(&lifeCycleContext{
		LifeCycle: &w.lifeCycleTracker,
	}, &c.Credentials)

	// Create temporary storage
	w.temporaryStorage, err = runtime.NewTemporaryStorage(c.TemporaryFolder)
	if err != nil {
		w.monitor.ReportError(err, "worker.New() failed to create TemporaryStorage")
		err = runtime.ErrFatalInternalError
		return
	}

	// Create webhookserver
	if c.WebHookServer != nil {
		w.webhookserver, err = webhookserver.NewServer(c.WebHookServer, &c.Credentials)
		if err != nil {
			w.monitor.ReportError(err, "worker.New() failed to setup webhookserver")
			err = runtime.ErrFatalInternalError
			return
		}
	}

	// Create environment
	w.environment = runtime.Environment{
		Monitor:          monitor,
		GarbageCollector: w.garbageCollector,
		TemporaryStorage: w.temporaryStorage,
		WebHookServer:    w.webhookserver,
		Worker:           &w.lifeCycleTracker,
		WorkerGroup:      c.WorkerOptions.WorkerGroup,
		WorkerID:         c.WorkerOptions.WorkerID,
		ProvisionerID:    c.WorkerOptions.ProvisionerID,
		WorkerType:       c.WorkerOptions.WorkerType,
		RootURL:          rootURL,
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

	// Create plugin manager
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
	// Adding supersederUrl to payload schema
	// NOTE: This can be removed when someday superseding is implemented in the queue
	if w.options.EnableSuperseding {
		payloadSchema.Properties["supersederUrl"] = schematypes.URI{
			Title: "Superseder URL",
			Description: util.Markdown(`
				URL to the superseder service, See [superseding documentation](https://docs.taskcluster.net` +
				`/reference/platform/taskcluster-queue/docs/superseding) for details.
			`),
		}
	}
	return payloadSchema
}

// ErrWorkerStoppedNow is used to communicate that the worker was forcefully
// stopped. This could also be triggered by a plugin or engine.
var ErrWorkerStoppedNow = errors.New("worker was interrupted by StopNow")

// Start process tasks, returns ErrWorkerStoppedNow if not stopped gracefully.
func (w *Worker) Start() error {
	// Ensure that we don't start running twice
	if !w.started.Do(nil) {
		panic("Worker.Start() cannot be called twice, worker cannot restart")
	}

	// When StoppingNow is called, we give the worker 5 min to stop, or exit 1
	// StoppingNow typically happens due to an internal error, it's no unlikely
	// that this internal error caused a livelock by failing to release a lock, etc.
	done := make(chan struct{})
	defer close(done)
	go func() {
		w.lifeCycleTracker.StoppingNow.Wait()

		select {
		case <-done:
		case <-time.After(5 * time.Minute):
			go w.monitor.ReportError(errors.New(
				"Worker.Start(): livelock detected - didn't stop 5 min after StopNow()",
			))
			time.Sleep(30 * time.Second)
			os.Exit(1)
		}
	}()

	for !w.lifeCycleTracker.StoppingGracefully.IsDone() {
		// Claim tasks
		N := w.options.Concurrency - w.activeTasks.Value()
		debug("queue.claimWork(%s, %s) with capacity: %d", w.options.ProvisionerID, w.options.WorkerType, N)
		claims, err := w.queue.ClaimWork(w.options.ProvisionerID, w.options.WorkerType, &tcqueue.ClaimWorkRequest{
			WorkerGroup: w.options.WorkerGroup,
			WorkerID:    w.options.WorkerID,
			Tasks:       int64(N),
		})
		if err != nil && w.lifeCycleTracker.StoppingGracefully.IsDone() {
			// NOTE: err == context.Canceled || err == context.DeadlineExceeded
			//       Should also work once taskcluster-client-go returns the context.Err()
			//       See PR: https://github.com/taskcluster/taskcluster-client-go/pull/31
			break // if canceled we stop gracefully (we don't care to report such an error)
		}
		if err != nil {
			w.monitor.ReportError(err, "failed to ClaimWork")
			w.plugin.ReportNonFatalError()
		}

		// If we have claims we MUST always handle, even if we have stopNow!
		if claims != nil {
			for _, claim := range claims.Tasks {
				// Start processing tasks
				debug("starting to process task: %s/%d", claim.Status.TaskID, claim.RunID)
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
		case <-w.lifeCycleTracker.StoppingGracefully.Done():
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
	if w.lifeCycleTracker.StoppingNow.IsDone() {
		return ErrWorkerStoppedNow
	}
	return nil
}

// anonymous struct from tcqueue.ClaimWorkResponse.Tasks
type taskClaim struct {
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
}

// Utility function to create a queue client object
func (w *Worker) newQueueClient(ctx context.Context, creds *tcclient.Credentials) client.Queue {
	q := tcqueue.New(creds)
	if w.queueBaseURL != "" {
		q.BaseURL = w.queueBaseURL
	}
	if ctx != nil {
		q.Context = ctx
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

	// If superseding is enabled, find superseding if one is available
	// NOTE: This can be removed when superseding is implemented in the queue
	if w.options.EnableSuperseding {
		var done func()
		claim, done = w.superseding(claim)
		defer done()
	}

	// Create monitor for this task
	monitor := w.monitor.WithTags(map[string]string{
		"taskId": claim.Status.TaskID,
		"runId":  strconv.Itoa(int(claim.RunID)),
	})
	monitor.Info("starting to process task")
	defer monitor.Info("done processing task")

	// Create task client
	q := w.newQueueClient(context.Background(), &tcclient.Credentials{
		ClientID:    claim.Credentials.ClientID,
		AccessToken: claim.Credentials.AccessToken,
		Certificate: claim.Credentials.Certificate,
	})

	// Convert task definition to interface{} form
	var jsontask interface{}
	rawTask, _ := json.Marshal(claim.Task)
	_ = json.Unmarshal(rawTask, jsontask)

	// Create a taskrun
	var payload map[string]interface{}
	if json.Unmarshal(claim.Task.Payload, &payload) != nil {
		panic("unable to parse payload as JSON, this shouldn't be possible")
	}
	run := taskrun.New(taskrun.Options{
		Environment:   w.environment,
		Engine:        w.engine,
		PluginManager: w.plugin,
		Monitor:       monitor.WithPrefix("taskrun"),
		Queue:         q,
		Payload:       payload,
		TaskInfo: runtime.TaskInfo{
			TaskID:   claim.Status.TaskID,
			RunID:    int(claim.RunID),
			Created:  time.Time(claim.Task.Created),
			Deadline: time.Time(claim.Task.Deadline),
			Expires:  time.Time(claim.Task.Expires),
			Scopes:   claim.Task.Scopes,
			Task:     jsontask,
		},
	})
	run.SetCredentials(
		claim.Credentials.ClientID,
		claim.Credentials.AccessToken,
		claim.Credentials.Certificate,
	)

	// runId as string for use in requests
	runID := strconv.Itoa(int(claim.RunID))

	// Start reclaiming
	stopReclaiming := make(chan struct{})
	reclaimingDone := make(chan struct{})
	go func() {
		defer close(reclaimingDone)
		takenUntil := time.Time(claim.TakenUntil)
		for {
			// Wait for reclaim delay, stop of reclaiming, or stopNow called
			select {
			case <-stopReclaiming:
				return
			case <-w.lifeCycleTracker.StoppingNow.Done():
				run.Abort(taskrun.WorkerShutdown)
				return
			case <-time.After(w.reclaimDelay(takenUntil)):
			}

			// Reclaim task
			debug("queue.reclaimTask(%s, %d)", claim.Status.TaskID, claim.RunID)
			result, err := q.ReclaimTask(claim.Status.TaskID, runID)
			if err != nil {
				if e, ok := err.(*tcclient.APICallException); ok && e.CallSummary.HTTPResponse.StatusCode == 409 {
					debug("queue.reclaimTask(%s, %d) -> 409, task was canceled")
					run.Abort(taskrun.TaskCanceled)
					return
				}
				monitor.ReportWarning(err, "failed to reclaim task")
				continue // Maybe we'll have more luck next time
			}

			// Update takenUntil and create a new queue client
			takenUntil = time.Time(result.TakenUntil)
			q = w.newQueueClient(context.Background(), asClientCredentials(result.Credentials))
			run.SetQueueClient(q) // update queue client on the run
			run.SetCredentials(
				result.Credentials.ClientID,
				result.Credentials.AccessToken,
				result.Credentials.Certificate,
			)
		}
	}()

	// Wait for taskrun to finish
	success, exception, reason := run.WaitForResult()

	// Stop reclaiming
	close(stopReclaiming)

	// Wait for reclaiming to end (we can't use q while it may be updated)
	<-reclaimingDone

	// Report task resolution
	debug("reporting task %s/%d resolved", claim.Status.TaskID, claim.RunID)
	var err error
	if exception {
		if reason != runtime.ReasonCanceled {
			_, err = q.ReportException(claim.Status.TaskID, runID, &tcqueue.TaskExceptionRequest{
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
	if e, ok := err.(httpbackoff.BadHttpResponseCode); ok && e.HttpResponseCode == 409 {
		monitor.Info("request conflict reporting task resolution, task was probably cancelled")
		err = nil // ignore error
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

// superseding returns any superseding task, and a function to be called when
// processed to resolve other superseded tasks.
func (w *Worker) superseding(claim taskClaim) (taskClaim, func()) {
	// Create monitor for any problems we run into
	m := w.monitor.WithPrefix("superseding")

	var payload map[string]interface{}
	if json.Unmarshal(claim.Task.Payload, &payload) != nil {
		// Do nothing, if there is an error, it'll show up later and logging will be
		// more natural.
		return claim, func() {}
	}

	// Take supersederUrl out of the payload, as it would break the payload
	// validation done in TaskRun. We attempt to hide superseding from the rest
	// of the worker implementation, so that it's only creating a hack here.
	supersederURL, hasSupersederURL := payload["supersederUrl"].(string)
	delete(payload, "supersederUrl")
	var err error
	claim.Task.Payload, err = json.Marshal(payload)
	if err != nil {
		panic(errors.Wrap(err, "failed to serialize data we know to be JSON"))
	}

	// Do nothing, if there is no supersederUrl
	if !hasSupersederURL || supersederURL == "" {
		return claim, func() {}
	}

	// Fetch list of superseding tasks from superseder
	g := got.New()
	r, err := g.Get(supersederURL + "?taskId=" + claim.Status.TaskID).Send()
	if err != nil {
		m.Warnf("failed to contact supersederUrl: '%s'", supersederURL)
		return claim, func() {}
	}

	var result struct {
		Supersedes []string `json:"supersedes"`
	}
	if json.Unmarshal(r.Body, &result) != nil {
		m.Warnf("failed to JSON parse result from supersederUrl: '%s'", supersederURL)
		return claim, func() {}
	}

	claimAttempts := make([]taskClaim, len(result.Supersedes))
	util.Spawn(len(result.Supersedes), func(i int) {
		taskID := result.Supersedes[i]
		// Don't attempt to reclaim the initial task
		if taskID == claim.Status.TaskID {
			return
		}
		// Get state of the task, to find runID
		s, qerr := w.queue.Status(taskID)
		if qerr != nil {
			m.WithTag("taskId", taskID).Infof("unable to get task status, error: %v", qerr)
			return
		}
		runID := len(s.Status.Runs) - 1
		if runID < 0 || s.Status.Runs[runID].State != "pending" {
			return
		}
		c, qerr := w.queue.ClaimTask(taskID, strconv.Itoa(runID), &tcqueue.TaskClaimRequest{
			WorkerID:    w.options.WorkerID,
			WorkerGroup: w.options.WorkerGroup,
		})
		if qerr != nil {
			m.WithTags(map[string]string{
				"taskId": taskID,
				"runId":  strconv.Itoa(runID),
			}).Debug("unable to claimTask from superseder, error: %v", qerr)
			return
		}
		claimAttempts[i] = taskClaim(*c)
	})

	// remove invalid claims
	claims := []taskClaim{claim}
	for _, claim := range claimAttempts {
		if claim.Status.TaskID != "" {
			claims = append(claims, claim)
		}
	}

	// Take the last claim we have, as the task we run
	claim = claims[len(claims)-1]
	claims = claims[:len(claims)-1]

	// Start a reclaiming loop, and finish off by resolving superseded
	var stopReclaiming atomics.Once
	var tasksResolved atomics.WaitGroup
	tasksResolved.Add(len(claims))
	ctx, cancel := context.WithCancel(context.Background())
	go util.Spawn(len(claims), func(i int) {
		defer tasksResolved.Done()
		taskID := claims[i].Status.TaskID
		runID := strconv.Itoa(int(claims[i].RunID))
		for {
			select {
			case <-time.After(10 * time.Minute):
				// reclaim claims[i]
				q := w.newQueueClient(ctx, asClientCredentials(claims[i].Credentials))
				result, qerr := q.ReclaimTask(taskID, runID)
				if qerr != nil {
					m.WithTags(map[string]string{
						"taskId": taskID,
						"runId":  runID,
					}).Warnf("failed reclaimTask error: %v", qerr)
					return
				}
				claims[i].Credentials = result.Credentials
				claims[i].TakenUntil = result.TakenUntil
			case <-stopReclaiming.Done():
				// resolve claims[i] as superseded
				q := w.newQueueClient(ctx, asClientCredentials(claims[i].Credentials))
				// TODO: Upload artifacts
				_, qerr := q.ReportException(taskID, runID, &tcqueue.TaskExceptionRequest{
					Reason: "superseded",
				})
				if qerr != nil {
					m.WithTags(map[string]string{
						"taskId": taskID,
						"runId":  runID,
					}).Warnf("failed to reportException with reason superseded, error: %v", qerr)
				}
				return
			}
		}
	})

	// Remove supersederUrl from payload, so that it doesn't create malformed-payload
	payload = make(map[string]interface{})
	if json.Unmarshal(claim.Task.Payload, &payload) == nil {
		delete(payload, "supersederUrl")
		claim.Task.Payload, err = json.Marshal(payload)
		if err != nil {
			panic(errors.Wrap(err, "failed to serialize data known to be JSON"))
		}
	}

	// Return primary claim, and done() function to stop reclaiming and resolve
	return claim, func() {
		// Call cancel() once to abort the Context used, after 90s as we don't
		// want to hang because of some bug...
		var cancelled atomics.Once
		defer cancelled.Do(cancel)
		go func() {
			select {
			case <-cancelled.Done():
				return
			case <-time.After(90 * time.Second):
				cancelled.Do(cancel)
			}
		}()
		// Stop reclaiming, this causes the other tasks to resolve superseded
		stopReclaiming.Do(nil)
		// Wait for tasks to be resolved
		tasksResolved.Wait()
	}
}

func asClientCredentials(c struct {
	AccessToken string `json:"accessToken"`
	Certificate string `json:"certificate"`
	ClientID    string `json:"clientId"`
}) *tcclient.Credentials {
	return &tcclient.Credentials{
		ClientID:    c.ClientID,
		AccessToken: c.AccessToken,
		Certificate: c.Certificate,
	}
}

// StopNow aborts current tasks resolving worker-shutdown, and causes Work()
// to return an error.
func (w *Worker) StopNow() {
	debug("Worker.StopNow() called")
	w.lifeCycleTracker.StopNow()
}

// StopGracefully stops claiming new tasks and returns nil from Work() when
// all currently running tasks are done.
func (w *Worker) StopGracefully() {
	debug("Worker.StopGracefully() called")
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
	if w.webhookserver != nil {
		w.webhookserver.Stop()
	}

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
		w.lifeCycleTracker.StoppingNow.Do(nil)
	}
}
