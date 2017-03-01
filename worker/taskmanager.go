package worker

import (
	"fmt"
	"sync"

	"github.com/taskcluster/taskcluster-client-go"
	tcqueue "github.com/taskcluster/taskcluster-client-go/queue"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/gc"
)

// Manager is resonsible for managing the entire task lifecyle from claiming the
// task, creating a sandbox environment, and reporting the results fo the execution.
// The manager will also be responsible for ensuring tasks do not run past their max run
// time and are aborted if a cancellation message is received.
type Manager struct {
	sync.RWMutex
	doneClaimingTasks  chan struct{}
	doneExecutingTasks chan struct{}
	config             *configType
	engine             engines.Engine
	environment        *runtime.Environment
	pluginManager      plugins.Plugin
	pluginOptions      *plugins.PluginOptions
	monitor            runtime.Monitor
	queue              QueueService
	provisionerID      string
	workerGroup        string
	workerID           string
	tasks              map[string]*TaskRun
	gc                 *gc.GarbageCollector
}

// Create a new instance of the task manager that will be responsible for claiming,
// executing, and resolving units of work (tasks).
func newTaskManager(
	config *configType, engine engines.Engine, pluginManager plugins.Plugin,
	environment *runtime.Environment, monitor runtime.Monitor, gc *gc.GarbageCollector,
) (*Manager, error) {
	queue := tcqueue.New(
		&tcclient.Credentials{
			ClientID:    config.Credentials.ClientID,
			AccessToken: config.Credentials.AccessToken,
			Certificate: config.Credentials.Certificate,
		},
	)

	if config.QueueBaseURL != "" {
		queue.BaseURL = config.QueueBaseURL
	}

	service := &queueService{
		capacity:         config.Capacity,
		interval:         config.PollingInterval,
		client:           queue,
		provisionerID:    config.ProvisionerID,
		workerGroup:      config.WorkerGroup,
		workerID:         config.WorkerID,
		workerType:       config.WorkerType,
		monitor:          monitor.WithPrefix("queue-service"),
		expirationOffset: config.ReclaimOffset,
		maxTasksToRun:    config.MaxTasksToRun,
	}

	m := &Manager{
		tasks:              make(map[string]*TaskRun),
		doneClaimingTasks:  make(chan struct{}),
		doneExecutingTasks: make(chan struct{}),
		config:             config,
		engine:             engine,
		environment:        environment,
		monitor:            monitor,
		queue:              service,
		provisionerID:      config.ProvisionerID,
		workerGroup:        config.WorkerGroup,
		workerID:           config.WorkerID,
		gc:                 gc,
	}

	m.pluginManager = pluginManager
	return m, nil
}

// Start will instruct the queue service to begin claiming work and will run task
// claims that are returned by the queue service.
func (m *Manager) Start() {
	tc := m.queue.Start()
	var wg sync.WaitGroup
	go func() {
		for t := range tc {
			wg.Add(1)
			go func(t *taskClaim) {
				defer wg.Done()
				m.run(t)
			}(t)
		}
		close(m.doneClaimingTasks)
		wg.Wait()
		close(m.doneExecutingTasks)
	}()
	return
}

// ImmediateStop should be called when the worker should aggressively terminate
// all running tasks and then gracefully terminate.
func (m *Manager) ImmediateStop() {
	m.queue.Stop()
	<-m.doneClaimingTasks

	m.Lock()
	defer m.Unlock()
	for _, task := range m.tasks {
		task.Abort()
	}
	<-m.doneExecutingTasks
}

// GracefulStop should be called when the worker should stop claiming new
// tasks, but wait for existing tasks to complete naturally
func (m *Manager) GracefulStop() {
	m.queue.Stop()
	<-m.doneExecutingTasks
}

// RunningTasks returns the list of task names that are currently running. This could
// be useful for determining the number of tasks currently running or snapshotting
// the current running task list at a moment in time.
func (m *Manager) RunningTasks() []string {
	m.RLock()
	defer m.RUnlock()

	tasks := []string{}
	for k := range m.tasks {
		tasks = append(tasks, k)
	}

	return tasks
}

// CancelTask will cancel a running task.  Typically this will be called when a Pulse
// message is received to cancel a task.  Calling this method will not resolve the task
// as it's assumed that this task was already resolved as cancelled by another system/client.
func (m *Manager) CancelTask(taskID string, runID int) {
	name := fmt.Sprintf("%s/%d", taskID, runID)

	m.RLock()
	defer m.RUnlock()

	t, exists := m.tasks[name]
	if !exists {
		return
	}

	t.Cancel()
	return
}

func (m *Manager) run(claim *taskClaim) {
	// Always do a best-effort GCing before we run a task
	if err := m.gc.Collect(); err != nil {
		m.monitor.Error("Failed to run garbage collector, error: ", err)
	}

	monitor := m.monitor.WithTags(map[string]string{
		"taskID": claim.taskID,
		"runID":  fmt.Sprintf("%d", claim.runID),
	})

	task, err := newTaskRun(m.config, claim, m.environment, m.engine, m.pluginManager, monitor)
	if err != nil {
		// This is a fatal call because creating a task run should never fail.
		monitor.WithTag("error", err.Error()).Panic("Could not successfully run the task")
	}

	err = m.registerTask(task)
	if err != nil {
		monitor.WithTag("error", err.Error()).Warn("Could not register task")
		panic(err)
	}

	defer m.deregisterTask(task)

	task.Run()
	return
}

func (m *Manager) registerTask(task *TaskRun) error {
	name := fmt.Sprintf("%s/%d", task.TaskID, task.RunID)
	m.monitor.Debugf("Registered task: %s", name)

	m.Lock()
	defer m.Unlock()

	_, exists := m.tasks[name]
	if exists {
		return fmt.Errorf("Cannot register task %s. Task already exists.", name)
	}

	m.tasks[name] = task
	return nil
}

func (m *Manager) deregisterTask(task *TaskRun) error {
	name := fmt.Sprintf("%s/%d", task.TaskID, task.RunID)
	m.monitor.Debugf("Deregistered task: %s", name)

	m.Lock()
	defer m.Unlock()

	_, exists := m.tasks[name]
	if !exists {
		return fmt.Errorf("Cannot deregister task %s. Task does not exist", name)
	}

	delete(m.tasks, name)
	m.queue.Done()
	return nil
}
