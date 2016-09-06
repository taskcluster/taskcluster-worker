package worker

import (
	"fmt"

	"github.com/Sirupsen/logrus"
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

// Worker is the center of taskcluster-worker and is responsible for managing resources, tasks,
// and host level events.
type Worker struct {
	log  *logrus.Entry
	done chan struct{}
	tm   *Manager
	sm   runtime.ShutdownManager
}

// New will create a worker given configuration matching the schema from
// ConfigSchema()
func New(config interface{}, environment *runtime.Environment) (*Worker, error) {
	// Validate and map configuration to c
	var c configType
	if err := schematypes.MustMap(ConfigSchema(), config, &c); err != nil {
		return nil, fmt.Errorf("Invalid configuration: %s", err)
	}

	// Ensure that engine confiuguration was provided for the engine selected
	if _, ok := c.Engines[c.Engine]; !ok {
		return nil, fmt.Errorf("Invalid configuration: The key 'engines.%s' must "+
			"be specified when engine '%s' is selected", c.Engine, c.Engine)
	}

	// Find engine provider (schema should ensure it exists)
	provider := engines.Engines()[c.Engine]
	engine, err := provider.NewEngine(engines.EngineOptions{
		Environment: environment,
		Log:         environment.Log.WithField("engine", c.Engine),
		Config:      c.Engines[c.Engine],
	})
	if err != nil {
		return nil, fmt.Errorf("Engine initialization failed, error: %s", err)
	}

	// Initialize plugin manager
	pm, err := plugins.NewPluginManager(plugins.PluginOptions{
		Environment: environment,
		Engine:      engine,
		Log:         environment.Log.WithField("plugin", "plugin-manager"),
		Config:      c.Plugins,
	})
	if err != nil {
		return nil, fmt.Errorf("Plugin initialization failed, error: %s", err)
	}

	tm, err := newTaskManager(
		&c, engine, pm, environment,
		environment.Log.WithField("component", "task-manager"),
	)
	if err != nil {
		return nil, err
	}

	return &Worker{
		log:  environment.Log.WithField("component", "worker"),
		tm:   tm,
		sm:   runtime.NewShutdownManager("local"),
		done: make(chan struct{}),
	}, nil
}

// Start will begin the worker cycle of claiming and executing tasks.  The worker
// will also being to respond to host level events such as shutdown notifications and
// resource depletion events.
func (w *Worker) Start() {
	w.log.Info("worker starting up")

	go w.tm.Start()

	select {
	case <-w.sm.WaitForShutdown():
	case <-w.done:
	}

	w.tm.Stop()
	return
}

// Stop is a convenience method for stopping the worker loop.  Usually the worker will not be
// stopped this way, but rather will listen for a shutdown event.
func (w *Worker) Stop() {
	close(w.done)
}
