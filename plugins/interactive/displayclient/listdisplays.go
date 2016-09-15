package displayclient

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	got "github.com/taskcluster/go-got"
	"github.com/taskcluster/taskcluster-worker/plugins/interactive/displayconsts"
)

// A Display structure holds information about a display returned from
// ListDisplays()
type Display struct {
	displayconsts.DisplayEntry
	socketURL string
}

// OpenDisplay will attempt to open a websocket and create a DisplayClient for
// the display.
func (d Display) OpenDisplay() (*DisplayClient, error) {
	return Dial(d.socketURL, d.Display)
}

// ListDisplays will fetch a list of displays offered by the socketURL.
func ListDisplays(socketURL string) ([]Display, error) {
	// Parse socketURL and get query string
	u, err := url.Parse(socketURL)
	if err != nil {
		return nil, fmt.Errorf("Invalid socketURL: %s, parsing error: %s",
			socketURL, err)
	}

	// Ensure the URL has http or https as scheme
	switch u.Scheme {
	case "ws":
		u.Scheme = "http"
	case "wss":
		u.Scheme = "https"
	}

	// Create a new got client
	g := got.New()
	g.Client = &http.Client{Timeout: 30 * time.Second}
	g.MaxSize = 10 * 1024 * 1024

	// Send a request
	res, err := g.Get(u.String()).Send()

	// If non-2xx error we try to parse the error
	if rerr, ok := err.(*got.BadResponseCodeError); ok {
		var errorMsg displayconsts.ErrorMessage
		if json.Unmarshal(rerr.Response.Body, &errorMsg) == nil {
			return nil, &errorMsg
		}
		return nil, fmt.Errorf("Failed to list displays got status code: %d",
			res.StatusCode)
	}
	// Otherwise we just wrap the error for better interpretation
	if err != nil {
		return nil, fmt.Errorf("Failed to list displays, error: %s", err)
	}

	// If no error, we try to parse the response
	var displays []Display
	err = json.Unmarshal(res.Body, &displays)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse JSON response, error: %s", err)
	}

	// Set socketURL on all display structures
	for i := range displays {
		displays[i].socketURL = socketURL
	}

	return displays, nil
}
