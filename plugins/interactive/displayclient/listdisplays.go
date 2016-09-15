package displayclient

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	got "github.com/taskcluster/go-got"
)

// ListDisplays will fetch a list of displays offered by the socketURL.
func ListDisplays(socketURL string) ([]Display, error) {
	// Parse socketURL and get query string
	u, err := url.Parse(socketURL)
	if err != nil {
		return nil, fmt.Errorf("Invalid socketURL: %s, parsing error: %s",
			socketURL, err)
	}

	// Ensure the URL has ws or wss as scheme
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	}

	// Create a new got client
	got := got.New()
	got.Client = &http.Client{Timeout: 30 * time.Second}
	got.MaxSize = 10 * 1024 * 1024

	// Send a request
	res, err := got.Get(u.String()).Send()

	// If non-2xx error we try to parse the error
	if rerr, ok := err.(*BadResponseCodeError); ok {
		var errorMsg displayconsts.ErrorMesssage
		if json.Unmarshal(rerr.Request.Body, &errorMsg) == nil {
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
	var displays []displayconsts.DisplayEntry
	err := json.Unmarshal(res.Body, &displays)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse JSON response, error: %s", err)
	}

	return displays, nil
}
