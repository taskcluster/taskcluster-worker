package taskManager

import (
	"github.com/Sirupsen/logrus"
	tcqueue "github.com/taskcluster/taskcluster-client-go/queue"
	"github.com/taskcluster/taskcluster-client-go/tcclient"
	"github.com/taskcluster/taskcluster-worker/config"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type Manager struct {
	Tasks         []*runtime.TaskContext
	MaxCapacity   int
	Engine        *engines.Engine
	Log           *logrus.Entry
	Queue         *queueService
	ProvisionerId string
	WorkerType    string
}

// Start the task manager and begin executing tasks.
func (m *Manager) Start() {

}

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
		WorkerType:       config.WorkerType,
		Log:              log.WithField("component", "Queue Service"),
		ExpirationOffset: config.QueueService.ExpirationOffset,
	}

	return &Manager{
		Engine:        engine,
		Log:           log,
		MaxCapacity:   config.Capacity,
		Queue:         service,
		ProvisionerId: config.ProvisionerId,
		WorkerType:    config.WorkerType,
	}
}
