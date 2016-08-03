// Package qemuguesttools implements the command that runs inside a QEMU VM.
// These guest tools are reponsible for fetching and executing the task command,
// as well as posting the log from the task command to the meta-data service.
//
// The guest tools are also responsible for polling the meta-data service for
// actions to do like list-folder, get-artifact or execute a new shell.
//
// The guest tools is pretty much the only way taskcluster-worker can talk to
// the guest virtual machine. As you can't execute processes inside a virtual
// machine without SSH'ing into it or something. That something is these
// guest tools.
package qemuguesttools

import (
	"github.com/taskcluster/taskcluster-worker/commands/extpoints"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

var debug = runtime.Debug("guesttools")

func init() {
	extpoints.CommandProviders.Register(cmd{}, "qemu-guest-tools")
}

type cmd struct{}

func (cmd) Summary() string {
	return "Run guest-tools, for use in VMs for the QEMU engine"
}

func (cmd) Usage() string {
	return `taskcluster-worker qemu-guest-tools start the guest tools that should
run inside the virtual machines used with QEMU engine.

Usage: taskcluster-worker qemu-guest-tools [options]

Options:
      --host <ip>  IP-address of meta-data server [default: 169.254.169.254].
  -h, --help       Show this screen.`
}

func (cmd) Execute(arguments map[string]interface{}) {
	host := arguments["--host"].(string)

	logger, _ := runtime.CreateLogger("info")
	log := logger.WithField("component", "qemu-guest-tools")

	g := new(host, log)
	go g.Run()
	// Process actions forever, this must run in the main thread as exiting the
	// main thread will cause the go program to exit.
	g.ProcessActions()
}
