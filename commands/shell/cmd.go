package shell

import (
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/gorilla/websocket"
	isatty "github.com/mattn/go-isatty"
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

usage: taskcluster-worker shell [options] <URL> [--] [<command>...]

options:
  -h --help     Show this screen.
`
}

var dialer = websocket.Dialer{
	HandshakeTimeout: interactive.ShellHandshakeTimeout,
	ReadBufferSize:   interactive.ShellMaxMessageSize,
	WriteBufferSize:  interactive.ShellMaxMessageSize,
}

type winSize struct {
	Rows    uint16
	Columns uint16
	XPixel  uint16 // unused
	YPixel  uint16 // unused
}

func (cmd) Execute(arguments map[string]interface{}) bool {
	URL := arguments["<URL>"].(string)
	command := arguments["<command>"].([]string)
	tty := isatty.IsTerminal(os.Stdout.Fd())

	// Parse URL
	u, err := url.Parse(URL)
	if err != nil {
		fmt.Println("Failed to parse URL, error: ", err)
		return false
	}
	qs := u.Query()

	// Set the command, if we have one
	if len(command) > 0 {
		qs["command"] = command
	}

	// Set tty=true if we're in a tty
	if tty {
		qs.Set("tty", "true")
	}

	// Update query string
	u.RawQuery = qs.Encode()

	// Connect to remove websocket
	ws, res, err := dialer.Dial(u.String(), nil)
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

	// Switch terminal to raw mode
	cleanup := func() {}
	if tty {
		cleanup = SetupRawTerminal(shell.SetSize)
	}

	// Connect pipes
	go ioext.CopyAndClose(shell.StdinPipe(), os.Stdin)
	go io.Copy(os.Stdout, shell.StdoutPipe())
	go io.Copy(os.Stderr, shell.StderrPipe())

	// Wait for shell to be done
	success, _ := shell.Wait()

	// If we were in a tty we let's restore state
	cleanup()

	return success
}
