package worker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	_ "github.com/taskcluster/taskcluster-worker/engines/mock"
)

type mockedQueueService struct {
	started bool
	stopped bool
	worker  *Worker
}

func (m *mockedQueueService) Start() <-chan *taskClaim {
	defer m.worker.Stop()
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

func TestStart(t *testing.T) {
	w, err := New(map[string]interface{}{
		"engine": "mock",
		"engines": map[string]interface{}{
			"mock": map[string]interface{}{},
		},
		"plugins": map[string]interface{}{
			"disabled": []string{},
		},
		"capacity": 1,
		"credentials": map[string]interface{}{
			"clientId":    "no-client",
			"accessToken": "absolutely-no-secret-here-mocked-it-out",
		},
		"pollingInterval": 2,
		"reclaimOffset":   90,
		"provisionerId":   "dummy-provisioner",
		"workerType":      "dummy-test-worker",
		"workerGroup":     "dummy-test-A",
		"workerId":        "dummy-test-B",
		"temporaryFolder": "/tmp/tc-worker-tmp-test-folder",
		"logLevel":        "info",
	}, nil)
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
