package displayclient

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/gorilla/websocket"
	"github.com/taskcluster/taskcluster-worker/plugins/interactive/displayconsts"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

var dialer = websocket.Dialer{
	HandshakeTimeout: displayconsts.DisplayHandshakeTimeout,
	ReadBufferSize:   displayconsts.DisplayMaxMessageSize,
	WriteBufferSize:  displayconsts.DisplayMaxMessageSize,
}

// Dial will open a websocket to socketURL, pass the display string to the
// server and return a DisplayClient implementing ioext.ReadWriteCloser using
// the websocket.
//
// Use ListDisplays() to find available displays.
func Dial(socketURL string, display string) (*DisplayClient, error) {
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

	// Set display
	q.Set("display", display)

	// Set querystring on url
	u.RawQuery = q.Encode()

	// Dial up to the constructed URL
	ws, res, err := dialer.Dial(u.String(), nil)

	// Upgrade failed then we most likely got some error response
	if err == websocket.ErrBadHandshake {
		// Attempt to read and parse the body (limit to 2MiB for sanity)
		data, _ := ioext.ReadAtMost(res.Body, 2*1024*1024)
		var errorMsg displayconsts.ErrorMessage
		perr := json.Unmarshal(data, &errorMsg)
		if perr == nil {
			return nil, &errorMsg
		}
		// return a generic error message if body parsing failed
		return nil, fmt.Errorf("Failed to connect to display, statusCode: %d",
			res.StatusCode)
	}
	if err != nil {
		return nil, fmt.Errorf("Failed to open websocket, error: %s", err)
	}

	// Return a new DisplayClient wrapping the websocket
	return New(ws), nil
}
