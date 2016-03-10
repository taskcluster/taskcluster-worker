package worker

import (
	"github.com/Sirupsen/logrus"
	"github.com/taskcluster/taskcluster-worker/config"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

// Worker is the center of taskcluster-worker and is responsible for managing resources, tasks,
// and host level events.
type Worker struct {
	logger *logrus.Entry
	done   chan struct{}
	tm     *Manager
	sm     runtime.ShutdownManager
}

// New will create a worker and task manager.
func New(config *config.Config, engine engines.Engine, environment *runtime.Environment, log *logrus.Entry) (*Worker, error) {
	tm, err := newTaskManager(config, engine, environment, log)
	if err != nil {
		return nil, err
	}

	return &Worker{
		logger: log,
		tm:     tm,
		sm:     runtime.NewShutdownManager("local"),
		done:   make(chan struct{}),
	}, nil
}

// Start will begin the worker cycle of claiming and executing tasks.  The worker
// will also being to respond to host level events such as shutdown notifications and
// resource depletion events.
func (w *Worker) Start() {
	w.logger.Info("worker starting up")

	go w.tm.Start()

	select {
	case <-w.sm.WaitForShutdown():
	case <-w.done:
	}

	w.tm.Stop()
	return
}

// stop is a convenience method for stopping the worker loop.  Usually the worker will not be
// stopped this way, but rather will listen for a shutdown event.
func (w *Worker) stop() {
	close(w.done)
}
