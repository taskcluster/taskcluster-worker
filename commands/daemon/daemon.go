package daemon

import (
	"fmt"
	"os"

	daemonize "github.com/takama/daemon"
	"github.com/taskcluster/taskcluster-worker/commands"
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
  taskcluster-worker daemon (install | run) <config-file>
  taskcluster-worker daemon (start | stop | remove)
`
}

func (cmd) Usage() string {
	return usage()
}

func (cmd) Execute(args map[string]interface{}) bool {
	srv, err := daemonize.New(name, description, dependencies...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return false
	}

	svc := &service{srv, args}
	status, err := svc.Manage()

	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n%v\n", status, err)
	}

	fmt.Println(status)
	return true
}
