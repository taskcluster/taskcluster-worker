//go:generate go-extpoints ./engines/extpoints/
//go:generate go-extpoints ./plugins/extpoints/
//go:generate go-import-subtree engines/ plugins/

package main

import (
	"log"
	"os"

	"github.com/docopt/docopt-go"
	"github.com/taskcluster/taskcluster-worker/engines/extpoints"
	logger "github.com/taskcluster/taskcluster-worker/log"
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
    --help  				Show this help screen.
    --version  				Display the version of go-import-subtree and exit.
    -e --engine <engine>  	Engine to use for task execution sandboxes.
    -l --logging <level>  	Set logging at <level>.
`

func main() {
	args, err := docopt.Parse(usage, nil, true, version, false, true)
	if err != nil {
		log.Fatalf("Error parsing arguments. %v", err)
	}

	e := args["--engine"]
	if e == nil {
		panic("Must supply engine type")
	}

	engine := e.(string)

	engineProvider := extpoints.EngineProviders.Lookup(engine)

	if engineProvider == nil {
		engineNames := extpoints.EngineProviders.Names()
		log.Fatalf("Must supply a valid engine.  Supported Engines %v", engineNames)
	}

	_, err = engineProvider(extpoints.EngineOptions{})
	if err != nil {
		panic(err)
	}

	options := map[string]interface{}{"engine": e}
	logger := logger.New(os.Stdout, logger.DEBUG, options)

	runtimeEnvironment := runtime.Environment{Logger: logger}
	runtimeEnvironment.Logger.Debug("Worker started up", nil)
}
