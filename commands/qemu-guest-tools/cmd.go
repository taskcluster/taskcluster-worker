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
	"io"
	"os"

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

The "run" (default) command will fetch a command to execute from the meta-data
service, upload the log and result as success/failed. The command will also
continously poll the meta-data service for actions, such as put-artifact,
list-folder or start an interactive shell.

The "post-log" command will upload <log-file> to the meta-data service. If - is
given it will read the log from standard input. This command is useful as
meta-data can handle more than one log stream, granted they might get mangled.

Usage:
  taskcluster-worker qemu-guest-tools [options] [run]
  taskcluster-worker qemu-guest-tools [options] post-log [--] <log-file>

Options:
      --host <ip>  IP-address of meta-data server [default: 169.254.169.254].
  -h, --help       Show this screen.`
}

func (cmd) Execute(arguments map[string]interface{}) bool {
	host := arguments["--host"].(string)

	logger, _ := runtime.CreateLogger("info")
	log := logger.WithField("component", "qemu-guest-tools")

	g := new(host, log)

	if arguments["post-log"].(bool) {
		logFile := arguments["<log-file>"].(string)
		var r io.Reader
		if logFile == "-" {
			r = os.Stdin
		} else {
			f, err := os.Open(logFile)
			if err != nil {
				return false
			}
			defer f.Close()
			r = f
		}
		w, done := g.CreateTaskLog()
		_, err := io.Copy(w, r)
		if err != nil {
			err = w.Close()
			<-done
		}
		return err == nil
	}

	go g.Run()
	// Process actions forever, this must run in the main thread as exiting the
	// main thread will cause the go program to exit.
	g.ProcessActions()

	return true
}
