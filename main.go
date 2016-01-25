//go:generate go-extpoints ./engines/extpoints/
//go:generate go-extpoints ./plugins/extpoints/
//go:generate go-import-subtree engines/ plugins/

package main

import (
	"fmt"
	"log"

	"github.com/docopt/docopt-go"
	"github.com/taskcluster/taskcluster-worker/engines/extpoints"
)

const version = "taskcluster-worker 0.0.1"
const usage = `
Usage: taskcluster-worker [options]
Runs a worker with the given options.
Options:
  -V, --version            Display the version of go-import-subtree and exit.
  -h, --help               Print this help information.
  -e, --engine <engine>    Execution engine to run tasks in
`

func validateEngine(engine string, engines map[string]extpoints.EngineProvider) bool {
	_, exists := engines[engine]
	return exists
}

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

	if validateEngine(engine, extpoints.EngineProviders.All()) == false {
		engineNames := extpoints.EngineProviders.Names()
		log.Fatalf("Must supply a valid engine.  Supported Engines %v", engineNames)
	}

	engineProvider := extpoints.EngineProviders.Lookup(engine)
	engineInstance, err := engineProvider(extpoints.EngineOptions{})
	if err != nil {
		panic(err)
	}

	fmt.Println(engineInstance)

}
