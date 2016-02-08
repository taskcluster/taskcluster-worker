package taskmgr

import (
	"github.com/Sirupsen/logrus"
	tcqueue "github.com/taskcluster/taskcluster-client-go/queue"
	"github.com/taskcluster/taskcluster-client-go/tcclient"
	"github.com/taskcluster/taskcluster-worker/config"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

// Manager is resonsible for managing the entire task lifecyle from claiming the
// task, creating a sandbox environment, and reporting the results fo the execution.
// The manager will also be responsible for ensuring tasks do not run past their max run
// time and are aborted if a cancellation message is received.
type Manager struct {
	// List of Tasks Contexts for running tasks
	Tasks []*runtime.TaskContextController
	// Maxmimum capacity that the worker is configured for.
	MaxCapacity   int
	Engine        *engines.Engine
	Log           *logrus.Entry
	Queue         *queueService
	ProvisionerId string
	WorkerGroup   string
	WorkerId      string
}

// Start the task manager and begin executing tasks.
func (m *Manager) Start() {
}

// Create a new instance of the task manager that will be responsible for claiming,
// executing, and resolving units of work (tasks).
func New(config *config.Config, engine *engines.Engine, log *logrus.Entry) *Manager {
	queue := tcqueue.New(
		&tcclient.Credentials{
			ClientId:    config.Taskcluster.ClientId,
			AccessToken: config.Taskcluster.AccessToken,
			Certificate: config.Taskcluster.Certificate,
		},
	)
	service := &queueService{
		client:           queue,
		ProvisionerId:    config.ProvisionerId,
		WorkerGroup:      config.WorkerGroup,
		Log:              log.WithField("component", "Queue Service"),
		ExpirationOffset: config.QueueService.ExpirationOffset,
	}

	return &Manager{
		Engine:        engine,
		Log:           log,
		MaxCapacity:   config.Capacity,
		Queue:         service,
		ProvisionerId: config.ProvisionerId,
		WorkerGroup:   config.WorkerGroup,
		WorkerId:      config.WorkerId,
	}
}
