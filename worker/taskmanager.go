package worker

import (
	"fmt"
	"sync"

	"github.com/Sirupsen/logrus"
	tcqueue "github.com/taskcluster/taskcluster-client-go/queue"
	"github.com/taskcluster/taskcluster-client-go/tcclient"
	"github.com/taskcluster/taskcluster-worker/config"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/plugins/extpoints"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

// Manager is resonsible for managing the entire task lifecyle from claiming the
// task, creating a sandbox environment, and reporting the results fo the execution.
// The manager will also be responsible for ensuring tasks do not run past their max run
// time and are aborted if a cancellation message is received.
type Manager struct {
	sync.RWMutex
	wg            sync.WaitGroup
	done          chan struct{}
	config        *config.Config
	engine        engines.Engine
	environment   *runtime.Environment
	pluginManager plugins.Plugin
	pluginOptions *extpoints.PluginOptions
	log           *logrus.Entry
	queue         QueueService
	provisionerID string
	workerGroup   string
	workerID      string
	tasks         map[string]*TaskRun
}

// Create a new instance of the task manager that will be responsible for claiming,
// executing, and resolving units of work (tasks).
func newTaskManager(config *config.Config, engine engines.Engine, environment *runtime.Environment, log *logrus.Entry) (*Manager, error) {
	queue := tcqueue.New(
		&tcclient.Credentials{
			ClientID:    config.Credentials.ClientID,
			AccessToken: config.Credentials.AccessToken,
			Certificate: config.Credentials.Certificate,
		},
	)

	queue.BaseURL = config.Taskcluster.Queue.URL

	service := &queueService{
		capacity:         config.Capacity,
		interval:         config.PollingInterval,
		client:           queue,
		provisionerID:    config.ProvisionerID,
		workerGroup:      config.WorkerGroup,
		workerID:         config.WorkerID,
		workerType:       config.WorkerType,
		log:              log.WithField("component", "Queue Service"),
		expirationOffset: config.QueueService.ExpirationOffset,
	}

	m := &Manager{
		tasks:         make(map[string]*TaskRun),
		done:          make(chan struct{}),
		config:        config,
		engine:        engine,
		environment:   environment,
		log:           log,
		queue:         service,
		provisionerID: config.ProvisionerID,
		workerGroup:   config.WorkerGroup,
		workerID:      config.WorkerID,
	}

	m.pluginOptions = &extpoints.PluginOptions{
		Environment: environment,
		Engine:      engine,
		Log:         log.WithField("component", "Plugin Manager"),
	}

	pm, err := extpoints.NewPluginManager([]string{"success", "artifacts", "env", "volume"}, *m.pluginOptions)
	if err != nil {
		log.WithField("error", err.Error()).Warn("Error creating task manager. Could not create plugin manager")
		return nil, err
	}

	m.pluginManager = pm
	return m, nil
}

// Start will instruct the queue service to begin claiming work and will run task
// claims that are returned by the queue service.
func (m *Manager) Start() {
	tc := m.queue.Start()
	go func() {
		for t := range tc {
			go m.run(t)
		}
		close(m.done)
	}()
	return
}

// Stop should be called when the worker should gracefully end the execution of
// all running tasks before completely shutting down.
func (m *Manager) Stop() {
	m.queue.Stop()
	<-m.done

	m.Lock()
	for _, task := range m.tasks {
		task.Abort()
	}

	m.Unlock()
	m.wg.Wait()
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
	log := m.log.WithFields(logrus.Fields{
		"taskID": claim.taskID,
		"runID":  claim.runID,
	})

	task, err := NewTaskRun(m.config, claim, m.environment, m.engine, m.pluginManager, log)
	if err != nil {
		// This is a fatal call because creating a task run should never fail.
		log.WithField("error", err.Error()).Fatal("Could not successfully run the task")
	}

	err = m.registerTask(task)
	if err != nil {
		log.WithField("error", err.Error()).Warn("Could not register task")
		panic(err)
	}

	defer m.deregisterTask(task)

	task.Run()
	return
}

func (m *Manager) registerTask(task *TaskRun) error {
	name := fmt.Sprintf("%s/%d", task.TaskID, task.RunID)
	m.log.Debugf("Registered task: %s", name)

	m.Lock()
	defer m.Unlock()

	m.wg.Add(1)
	_, exists := m.tasks[name]
	if exists {
		return fmt.Errorf("Cannot register task %s. Task already exists.", name)
	}

	m.tasks[name] = task
	return nil
}

func (m *Manager) deregisterTask(task *TaskRun) error {
	name := fmt.Sprintf("%s/%d", task.TaskID, task.RunID)
	m.log.Debugf("Deregistered task: %s", name)

	m.Lock()
	defer m.Unlock()

	_, exists := m.tasks[name]
	if !exists {
		return fmt.Errorf("Cannot deregister task %s. Task does not exist", name)
	}

	delete(m.tasks, name)
	m.queue.Done()
	m.wg.Done()
	return nil
}
