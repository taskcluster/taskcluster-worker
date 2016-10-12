package shellserver

import (
	"fmt"
	"net/http"
	"os"
	goruntime "runtime"
	"time"

	graceful "gopkg.in/tylerb/graceful.v1"

	"github.com/taskcluster/taskcluster-worker/commands"
	"github.com/taskcluster/taskcluster-worker/plugins/interactive"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

var defaultShell = "sh"

func init() {
	if goruntime.GOOS == "windows" {
		defaultShell = "cmd.exe"
	}
	commands.Register("shell-server", cmd{})
}

type cmd struct{}

func (cmd) Summary() string {
	return "Host an interactive shell-server on localhost"
}

func (cmd) Usage() string {
	return `
taskcluster-worker shell-server will open a websocket to an interactive task, start
a shell and expose it in your terminal. This is similar to using an SSH client.

usage: taskcluster-worker shell-server [options]

options:
  -p --port <PORT>        Port to listen on [default: 2222].
  -s --shell <SHELL>      Default shell to use [default: ` + defaultShell + `].
     --log-level <level>  Log level [default: INFO].
  -h --help               Show this screen.
`
}

func (cmd) Execute(args map[string]interface{}) bool {
	log, err := runtime.CreateLogger(args["--log-level"].(string))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid log-level, error: %s", err)
		return false
	}

	// Create shell server
	shellServer := interactive.NewShellServer(
		newExecShell, log.WithField("component", "shell-server"),
	)

	// Setup server
	server := graceful.Server{
		Timeout: 35 * time.Second,
		Server: &http.Server{
			Addr:    fmt.Sprintf("127.0.0.1:%s", args["--port"].(string)),
			Handler: shellServer,
		},
		NoSignalHandling: false, // abort on sigint and sigterm
	}

	server.ListenAndServe()
	shellServer.Abort()
	shellServer.WaitAndClose()

	return true
}
