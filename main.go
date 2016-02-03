//go:generate go-extpoints ./engines/extpoints/
//go:generate go-extpoints ./plugins/extpoints/
//go:generate go-import-subtree engines/ plugins/

package main

import (
	"fmt"
	"os"

	"github.com/docopt/docopt-go"
	"github.com/taskcluster/taskcluster-worker/engines/extpoints"
	"github.com/taskcluster/taskcluster-worker/runtime"
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
    taskcluster-worker --engine <engine> --logging <level>

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

	logger, err := runtime.CreateLogger(args["--logging-level"])
	if err != nil {
		os.Stderr.WriteString(err.Error())
		os.Exit(1)
	}

	e := args["--engine"]
	engine := e.(string)

	engineProvider := extpoints.EngineProviders.Lookup(engine)

	if engineProvider == nil {
		engineNames := extpoints.EngineProviders.Names()
		logger.Fatalf("Must supply a valid engine.  Supported Engines %v", engineNames)
	}

	runtimeEnvironment := runtime.Environment{Log: logger}

	_, err = engineProvider(&extpoints.EngineOptions{Environment: &runtimeEnvironment})
	if err != nil {
		logger.Fatal(err.Error())
	}

	runtimeEnvironment.Log.Info("Worker started up")
}
