package shellclient

import (
	"fmt"
	"net/url"

	"github.com/gorilla/websocket"
	"github.com/taskcluster/taskcluster-worker/plugins/interactive/shellconsts"
)

var dialer = websocket.Dialer{
	HandshakeTimeout: shellconsts.ShellHandshakeTimeout,
	ReadBufferSize:   shellconsts.ShellMaxMessageSize,
	WriteBufferSize:  shellconsts.ShellMaxMessageSize,
}

// Dial will open a websocket to socketURL giving command and tty to the server.
// If no command is given the server should open its default shell in a human
// usable configuration.
func Dial(socketURL string, command []string, tty bool) (*ShellClient, error) {
	// Parse socketURL and get query string
	u, err := url.Parse(socketURL)
	if err != nil {
		return nil, fmt.Errorf("Invalid socketURL: %s, parsing error: %s",
			socketURL, err)
	}
	q := u.Query()

	// Ensure the URL has ws or wss as scheme
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	}

	// Set command arguments overwriting any existing querystring values
	q.Del("command")
	if len(command) > 0 {
		for _, arg := range command {
			q.Add("command", arg)
		}
	}

	// Set tty true or false
	if tty {
		q.Set("tty", "true")
	} else {
		q.Set("tty", "false")
	}

	// Set querystring on url
	u.RawQuery = q.Encode()

	// Dial up to the constructed URL
	ws, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		return nil, err
	}

	// Return a new ShellClient wrapping the websocket
	return New(ws), nil
}
