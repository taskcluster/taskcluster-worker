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
	return `
taskcluster-worker qemu-guest-tools start the guest tools that should run inside
the virtual machines used with QEMU engine.

usage: taskcluster-worker qemu-guest-tools [options]

options:
     --host <ip>  IP-address of meta-data server [default: 169.254.169.254].
  -h --help      	Show this screen.
`
}

func (cmd) Execute(arguments map[string]interface{}) {
	host := arguments["--host"].(string)

	logger, _ := runtime.CreateLogger("info")
	log := logger.WithField("component", "qemu-guest-tools")

	g := new(host, log)
	g.Run()
}
