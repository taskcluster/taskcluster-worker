package worker

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/taskcluster/taskcluster-worker/config"
	"github.com/taskcluster/taskcluster-worker/engines/extpoints"
	_ "github.com/taskcluster/taskcluster-worker/engines/mock"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/gc"
)

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

	start := time.Now()
	time.AfterFunc(2*time.Second, w.stop)

	err = w.Start()
	if err != nil {
		t.Fatal(err.Error())
	}

	d := time.Since(start).Seconds()
	// Worker should not have been running for longer than 2 seconds +- a few ms
	assert.True(t, d >= 2 && d <= 2.5)
}
