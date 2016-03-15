//go:generate go-extpoints ./engines/extpoints/
//go:generate go-extpoints ./plugins/extpoints/
//go:generate go-import-subtree engines/ plugins/

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Sirupsen/logrus"
	"github.com/docopt/docopt-go"
	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-worker/config"
	"github.com/taskcluster/taskcluster-worker/engines/extpoints"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/worker"
)

const version = "taskcluster-worker 0.0.1"
const usage = `
TaskCluster worker
This worker is meant to be used with the taskcluster platform for the execution and
resolution of tasks.

  Usage:
    taskcluster-worker --help
    taskcluster-worker --version
    taskcluster-worker --engine <engine>
    taskcluster-worker --engine <engine> --logging-level <level>

  Options:
    --help  						Show this help screen.
    --version  						Display the version of go-import-subtree and exit.
    -e --engine <engine>  			Engine to use for task execution sandboxes.
    -l --logging-level <level>  	Set logging at <level>.
`

func main() {
	args, err := docopt.Parse(usage, nil, true, version, false, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing arguments. %v", err)
		os.Exit(1)
	}

	var level string
	if l := args["--logging-level"]; l != nil {
		level = l.(string)
	}
	logger, err := runtime.CreateLogger(level)
	if err != nil {
		os.Stderr.WriteString(err.Error())
		os.Exit(1)
	}

	e := args["--engine"]
	engineName := e.(string)

	engineProvider := extpoints.EngineProviders.Lookup(engineName)

	if engineProvider == nil {
		engineNames := extpoints.EngineProviders.Names()
		logger.Fatalf("Must supply a valid engine.  Supported Engines %v", engineNames)
	}

	tempPath := filepath.Join(os.TempDir(), slugid.V4())
	tempStorage, err := runtime.NewTemporaryStorage(tempPath)
	runtimeEnvironment := &runtime.Environment{
		Log:              logger,
		TemporaryStorage: tempStorage,
	}

	engine, err := engineProvider.NewEngine(extpoints.EngineOptions{
		Environment: runtimeEnvironment,
		Log:         logger.WithField("engine", engineName),
	})
	if err != nil {
		logger.Fatal(err.Error())
	}

	// TODO (garndt): Need to load up a real config in the future
	config := &config.Config{
		Credentials: struct {
			AccessToken string `json:"accessToken"`
			Certificate string `json:"certificate"`
			ClientID    string `json:"clientID"`
		}{
			AccessToken: os.Getenv("TASKCLUSTER_ACCESS_TOKEN"),
			Certificate: os.Getenv("TASKCLUSTER_CERTIFICATE"),
			ClientID:    os.Getenv("TASKCLUSTER_CLIENT_ID"),
		},
		Capacity:      5,
		ProvisionerID: "dummy-test-provisioner",
		WorkerGroup:   "dummy-test-group",
		WorkerType:    "dummy-test-type",
		WorkerID:      "dummy-test-worker",
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
		logger.Fatalf("Could not create worker. %s", err)
	}

	w.Start()
}
