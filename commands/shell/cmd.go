package shell

import (
	"fmt"
	"os"

	"github.com/gorilla/websocket"
	"github.com/taskcluster/taskcluster-worker/commands/extpoints"
	"github.com/taskcluster/taskcluster-worker/plugins/interactive"
	"github.com/taskcluster/taskcluster-worker/plugins/interactive/shellclient"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

func init() {
	extpoints.CommandProviders.Register(cmd{}, "shell")
}

type cmd struct{}

func (cmd) Summary() string {
	return "Open interactive shell in your terminal"
}

func (cmd) Usage() string {
	return `
taskcluster-worker shell will open a websocket to an interactive task, start
a shell and expose it in your terminal. This is similar to using an SSH client.

usage: taskcluster-worker shell [options] [--] <URL>

options:
  -h --help     Show this screen.
`
}

var dialer = websocket.Dialer{
	HandshakeTimeout: interactive.ShellHandshakeTimeout,
	ReadBufferSize:   interactive.ShellMaxMessageSize,
	WriteBufferSize:  interactive.ShellMaxMessageSize,
}

func (cmd) Execute(arguments map[string]interface{}) bool {
	URL := arguments["<URL>"].(string)

	// Connect to remove websocket
	ws, res, err := dialer.Dial(URL, nil)
	if err == websocket.ErrBadHandshake {
		fmt.Println("Failed to connect, status: ", res.StatusCode)
		return false
	}
	if err != nil {
		fmt.Println("Failed to connect, error: ", err)
		return false
	}

	// Create shell client
	shell := shellclient.New(ws)

	// Connect pipes
	go ioext.CopyAndClose(shell.StdinPipe(), os.Stdin)
	go ioext.CopyAndClose(os.Stdout, shell.StdoutPipe())
	go ioext.CopyAndClose(os.Stderr, shell.StderrPipe())

	// Wait for shell to be done
	success, _ := shell.Wait()

	return success
}
