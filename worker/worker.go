package worker

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/Sirupsen/logrus"
	schematypes "github.com/taskcluster/go-schematypes"
	tcclient "github.com/taskcluster/taskcluster-client-go"
	"github.com/taskcluster/taskcluster-client-go/auth"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
	"github.com/taskcluster/taskcluster-worker/runtime/gc"
	"github.com/taskcluster/taskcluster-worker/runtime/webhookserver"
)

// Worker is the center of taskcluster-worker and is responsible for managing resources, tasks,
// and host level events.
type Worker struct {
	monitor runtime.Monitor
	done    chan struct{}
	tm      *Manager
	sm      runtime.ShutdownManager
	env     *runtime.Environment
	server  *webhookserver.LocalServer
}

// New will create a worker given configuration matching the schema from
// ConfigSchema(). The log parameter is optional and if nil is given a default
// logrus logger will be used.
func New(config interface{}, log *logrus.Logger) (*Worker, error) {
	// Validate and map configuration to c
	var c configType
	if err := schematypes.MustMap(ConfigSchema(), config, &c); err != nil {
		return nil, fmt.Errorf("Invalid configuration: %s", err)
	}

	// Create temporary folder
	err := os.RemoveAll(c.TemporaryFolder)
	if err != nil {
		return nil, fmt.Errorf("Failed to remove temporaryFolder: %s, error: %s",
			c.TemporaryFolder, err)
	}
	tempStorage, err := runtime.NewTemporaryStorage(c.TemporaryFolder)
	if err != nil {
		return nil, fmt.Errorf("Failed to create temporary folder, error: %s", err)
	}

	// Create logger
	if log == nil {
		log = logrus.New()
	}
	log.Level, _ = logrus.ParseLevel(c.LogLevel)

	// Setup WebHookServer
	localServer, err := webhookserver.NewLocalServer(
		net.ParseIP(c.ServerIP), c.ServerPort,
		c.NetworkInterface, c.ExposedPort,
		c.DNSDomain,
		c.DNSSecret,
		c.TLSCertificate,
		c.TLSKey,
		time.Duration(c.MaxLifeCycle)*time.Second,
	)
	if err != nil {
		return nil, err
	}

	// Setup monitor
	tags := map[string]string{
		"provisionerId": c.ProvisionerID,
		"workerType":    c.WorkerType,
		"workerGroup":   c.WorkerGroup,
		"workerId":      c.WorkerID,
	}
	var monitor runtime.Monitor
	if c.MonitorProject != "" {
		a := auth.New(&tcclient.Credentials{
			ClientID:    c.Credentials.ClientID,
			AccessToken: c.Credentials.AccessToken,
			Certificate: c.Credentials.Certificate,
		})
		monitor = runtime.NewMonitor(c.MonitorProject, a, c.LogLevel, tags)
	} else {
		monitor = runtime.NewLoggingMonitor(c.LogLevel, tags)
	}

	// Create environment
	gc := gc.New(c.TemporaryFolder, c.MinimumDiskSpace, c.MinimumMemory)
	env := &runtime.Environment{
		GarbageCollector: gc,
		TemporaryStorage: tempStorage,
		WebHookServer:    localServer,
		Monitor:          monitor,
	}

	// Ensure that engine confiuguration was provided for the engine selected
	if _, ok := c.Engines[c.Engine]; !ok {
		return nil, fmt.Errorf("Invalid configuration: The key 'engines.%s' must "+
			"be specified when engine '%s' is selected", c.Engine, c.Engine)
	}

	// Find engine provider (schema should ensure it exists)
	provider := engines.Engines()[c.Engine]
	engine, err := provider.NewEngine(engines.EngineOptions{
		Environment: env,
		Monitor:     env.Monitor.WithPrefix("engine").WithTag("engine", c.Engine),
		Config:      c.Engines[c.Engine],
	})
	if err != nil {
		return nil, fmt.Errorf("Engine initialization failed, error: %s", err)
	}

	// Initialize plugin manager
	pm, err := plugins.NewPluginManager(plugins.PluginOptions{
		Environment: env,
		Engine:      engine,
		Monitor:     env.Monitor.WithPrefix("plugin").WithTag("plugin", "plugin-manager"),
		Config:      c.Plugins,
	})
	if err != nil {
		return nil, fmt.Errorf("Plugin initialization failed, error: %s", err)
	}

	tm, err := newTaskManager(
		&c, engine, pm, env,
		env.Monitor.WithPrefix("task-manager"), gc,
	)
	if err != nil {
		return nil, err
	}

	return &Worker{
		monitor: env.Monitor.WithPrefix("worker"),
		tm:      tm,
		sm:      runtime.NewShutdownManager("local"),
		env:     env,
		server:  localServer,
		done:    make(chan struct{}),
	}, nil
}

// Start will begin the worker cycle of claiming and executing tasks.  The worker
// will also being to respond to host level events such as shutdown notifications and
// resource depletion events.
func (w *Worker) Start() {
	w.monitor.Info("worker starting up")

	// Ensure that server is stopping gracefully
	serverStopped := atomics.NewBool(false)
	go func() {
		err := w.server.ListenAndServe()
		if !serverStopped.Get() {
			w.monitor.Errorf("ListenAndServe failed for webhookserver, error: %s", err)
		}
	}()

	go w.tm.Start()

	select {
	case <-w.tm.doneExecutingTasks:
	case <-w.sm.WaitForShutdown():
	case <-w.done:
	}

	w.tm.ImmediateStop()

	// Allow server to stop
	serverStopped.Set(true)
	w.server.Stop()
	return
}

// ImmediateStop is a convenience method for stopping the worker loop.  Usually
// the worker will not be stopped this way, but rather will listen for a
// shutdown event.
func (w *Worker) ImmediateStop() {
	close(w.done)
}

// GracefulStop will allow the worker to complete its running task, before stopping.
func (w *Worker) GracefulStop() {
	w.tm.GracefulStop()
}

// PayloadSchema returns the payload schema for this worker.
func (w *Worker) PayloadSchema() schematypes.Object {
	payloadSchema, err := schematypes.Merge(
		w.tm.engine.PayloadSchema(),
		w.tm.pluginManager.PayloadSchema(),
	)
	if err != nil {
		panic(fmt.Sprintf(
			"Conflicting plugin and engine payload properties, error: %s", err,
		))
	}
	return payloadSchema
}
