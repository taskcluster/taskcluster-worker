// Package qemuguesttools implements the command that runs inside a QEMU VM.
// These guest tools are responsible for fetching and executing the task
// command, as well as posting the log from the task command to the meta-data
// service.
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
	"fmt"
	"io"
	"io/ioutil"
	"os"

	yaml "gopkg.in/yaml.v2"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/commands"
	"github.com/taskcluster/taskcluster-worker/runtime/monitoring"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

var debug = util.Debug("guesttools")

func init() {
	commands.Register("qemu-guest-tools", cmd{})
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
continuously poll the meta-data service for actions, such as put-artifact,
list-folder or start an interactive shell.

The "post-log" command will upload <log-file> to the meta-data service. If - is
given it will read the log from standard input. This command is useful as
meta-data can handle more than one log stream, granted they might get mangled.

Usage:
  taskcluster-worker qemu-guest-tools [options] [run]
  taskcluster-worker qemu-guest-tools [options] post-log [--] <log-file>

Options:
  -c, --config <file>  Load YAML configuration for file.
      --host <ip>      IP-address of meta-data server [default: 169.254.169.254].
  -h, --help           Show this screen.

Configuration:
  entrypoint: []                  # Wrapper command if any
  env:                            # Default environment variables
    HOME:     "/home/worker"
  shell:      ["bash", "-bash"]   # Default interactive system shell
  user:       "worker"            # User to run commands under
  workdir:    "/home/worker"      # Current working directory for commands
`
}

func (cmd) Execute(arguments map[string]interface{}) bool {
	monitor := monitoring.NewLoggingMonitor("info", nil, "").WithTag("component", "qemu-guest-tools")

	host := arguments["--host"].(string)
	configFile, _ := arguments["--config"].(string)

	// Load configuration
	var C config
	if configFile != "" {
		data, err := ioutil.ReadFile(configFile)
		if err != nil {
			monitor.Panicf("Failed to read configFile: %s, error: %s", configFile, err)
		}
		var c interface{}
		if err := yaml.Unmarshal(data, &c); err != nil {
			monitor.Panicf("Failed to parse configFile: %s, error: %s", configFile, err)
		}
		c = convertSimpleJSONTypes(c)
		if err := configSchema.Validate(c); err != nil {
			monitor.Panicf("Invalid configFile: %s, error: %s", configFile, err)
		}
		schematypes.MustValidateAndMap(configSchema, c, &C)
	}

	// Create guest tools
	g := new(C, host, monitor)

	if arguments["post-log"].(bool) {
		logFile := arguments["<log-file>"].(string)
		var r io.Reader
		if logFile == "-" {
			r = os.Stdin
		} else {
			f, err := os.Open(logFile)
			if err != nil {
				monitor.Error("Failed to open log-file, error: ", err)
				return false
			}
			defer f.Close()
			r = f
		}
		w, done := g.CreateTaskLog()
		_, err := io.Copy(w, r)
		if err != nil {
			monitor.Error("Failed to post entire log, error: ", err)
		} else {
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

func convertSimpleJSONTypes(val interface{}) interface{} {
	switch val := val.(type) {
	case []interface{}:
		r := make([]interface{}, len(val))
		for i, v := range val {
			r[i] = convertSimpleJSONTypes(v)
		}
		return r
	case map[interface{}]interface{}:
		r := make(map[string]interface{})
		for k, v := range val {
			s, ok := k.(string)
			if !ok {
				s = fmt.Sprintf("%v", k)
			}
			r[s] = convertSimpleJSONTypes(v)
		}
		return r
	case int:
		return float64(val)
	default:
		return val
	}
}
