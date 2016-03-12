package worker

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/taskcluster/taskcluster-worker/config"
	"github.com/taskcluster/taskcluster-worker/engines/extpoints"
	_ "github.com/taskcluster/taskcluster-worker/engines/mock"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/gc"
)

type mockedQueueService struct {
	started bool
	stopped bool
	worker  *Worker
}

func (m *mockedQueueService) Start() <-chan *taskClaim {
	defer m.worker.stop()
	defer close(m.worker.tm.done)
	m.started = true
	return make(chan *taskClaim)
}

func (m *mockedQueueService) Stop() {
	m.stopped = true
	return
}

func (mockedQueueService) Done() {
	return
}

func newWorker(t *testing.T, c *config.Config) (*Worker, error) {
	logger, err := runtime.CreateLogger(os.Getenv("LOGGING_LEVEL"))
	e := &runtime.Environment{
		GarbageCollector: &gc.GarbageCollector{},
		Log:              logger,
	}
	engineProvider := extpoints.EngineProviders.Lookup("mock")
	if engineProvider == nil {
		t.Fatalf("Couldn't find EngineProvider: %s", "mock")
	}
	// Create Engine instance
	engine, err := engineProvider.NewEngine(extpoints.EngineOptions{
		Environment: e,
		Log:         logger.WithField("component", "environment"),
	})
	if err != nil {
		t.Fatal(err)
	}
	return New(c, engine, e, logger.WithField("component", "worker"))
}

func TestStart(t *testing.T) {
	c := &config.Config{
		PollingInterval: 2,
	}
	w, err := newWorker(t, c)
	if err != nil {
		t.Fatal(err)
	}

	mockedQueue := &mockedQueueService{
		worker: w,
	}
	w.tm.queue = mockedQueue

	w.Start()

	// Assure that the queue service was started/stopped
	//as a sign the worker successfully started
	assert.True(t, mockedQueue.started, "Queue Service should have been started")
	assert.True(t, mockedQueue.stopped, "Queue Service should have been stopped")
}
