package daemon

import (
	"fmt"
	"os"

	daemonize "github.com/takama/daemon"
	"github.com/taskcluster/taskcluster-worker/commands"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

const (
	// name of the service
	name        = "taskcluster-worker"
	description = "Taskcluster worker"
)

var dependencies = []string{}

func init() {
	commands.Register("daemon", cmd{})
}

type cmd struct{}

func (cmd) Summary() string {
	return "Run as a daemon."
}

func usage() string {
	return `Usage:
  taskcluster-worker daemon (install | run) <engine> [--logging-level <level>]
  taskcluster-worker daemon (start | stop | remove)

Options:
  -l <level>, --logging-level=<level>   Logging level [default: info]
`
}

func (cmd) Usage() string {
	return usage()
}

func (cmd) Execute(args map[string]interface{}) bool {
	// set up logger
	var level string
	if l := args["--logging-level"]; l != nil {
		level = l.(string)
	}
	logger, err := runtime.CreateLogger(level)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	srv, err := daemonize.New(name, description, dependencies...)
	if err != nil {
		logger.Fatal("Error: ", err)
	}

	svc := &service{srv, args, logger}
	status, err := svc.Manage()

	if err != nil {
		logger.Fatalf("%s\n%v", status, err)
	}

	logger.Info(status)
	return true
}
