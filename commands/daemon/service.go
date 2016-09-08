package daemon

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/takama/daemon"
	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-worker/config"
	"github.com/taskcluster/taskcluster-worker/engines/extpoints"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/worker"
)

// service has embedded daemon
type service struct {
	daemon.Daemon
	args map[string]interface{}
	log  *logrus.Logger
}

func (svc *service) Run() (string, error) {
	logger := svc.log
	err := setupSyslog(logger)
	if err != nil {
		return "Could not create syslog", err
	}

	// Find engine provider
	engineName := svc.args["<engine>"].(string)
	engineProvider := extpoints.EngineProviders.Lookup(engineName)
	if engineProvider == nil {
		engineNames := extpoints.EngineProviders.Names()
		return "Engine not found", fmt.Errorf("Must supply a valid engine. Supported Engines %v", engineNames)
	}

	// Create a temporary folder
	tempPath := filepath.Join(os.TempDir(), slugid.Nice())
	tempStorage, err := runtime.NewTemporaryStorage(tempPath)
	runtimeEnvironment := &runtime.Environment{
		Log:              logger,
		TemporaryStorage: tempStorage,
	}

	// Initialize the engine
	engine, err := engineProvider.NewEngine(extpoints.EngineOptions{
		Environment: runtimeEnvironment,
		Log:         logger.WithField("engine", engineName),
	})
	if err != nil {
		return "Could not create engine", err
	}

	// TODO (garndt): Need to load up a real config in the future
	config := &config.Config{
		Credentials: struct {
			AccessToken string `json:"accessToken"`
			Certificate string `json:"certificate"`
			ClientID    string `json:"clientId"`
		}{
			AccessToken: os.Getenv("TASKCLUSTER_ACCESS_TOKEN"),
			Certificate: os.Getenv("TASKCLUSTER_CERTIFICATE"),
			ClientID:    os.Getenv("TASKCLUSTER_CLIENT_ID"),
		},
		Taskcluster: struct {
			Queue struct {
				URL string `json:"url,omitempty"`
			} `json:"queue,omitempty"`
		}{
			Queue: struct {
				URL string `json:"url,omitempty"`
			}{
				URL: "https://queue.taskcluster.net/v1/",
			},
		},
		Capacity:      5,
		ProvisionerID: "test-dummy-provisioner",
		WorkerType:    "dummy-worker-tc",
		WorkerGroup:   "test-dummy-workers",
		WorkerID:      "dummy-worker-tc",
		QueueService: struct {
			ExpirationOffset int `json:"expirationOffset"`
		}{
			ExpirationOffset: 300,
		},
		PollingInterval: 10,
	}

	l := logger.WithFields(logrus.Fields{
		"workerID":      config.WorkerID,
		"workerType":    config.WorkerType,
		"workerGroup":   config.WorkerGroup,
		"provisionerID": config.ProvisionerID,
	})

	w, err := worker.New(config, engine, runtimeEnvironment, l)
	if err != nil {
		return "Could not create worker", err
	}

	sigTerm := make(chan os.Signal, 1)
	signal.Notify(sigTerm, syscall.SIGTERM)
	go func() {
		<-sigTerm
		w.Stop()
	}()
	w.Start()
	return "Worker successfully started", nil
}

// Manage by daemon commands or run the daemon
func (svc *service) Manage() (string, error) {
	// if received any kind of command, do it
	if svc.args["install"].(bool) {
		args := []string{"daemon", "run", svc.args["<engine>"].(string)}
		for _, a := range []string{"--logging-level"} {
			args = append(args, a, svc.args[a].(string))
		}
		return svc.Install(args...)
	}

	if svc.args["remove"].(bool) {
		return svc.Remove()
	}

	if svc.args["start"].(bool) {
		return svc.Start()
	}

	if svc.args["stop"].(bool) {
		return svc.Stop()
	}

	if svc.args["run"].(bool) {
		return svc.Run()
	}

	return usage(), nil
}
