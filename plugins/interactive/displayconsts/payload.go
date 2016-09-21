package displayconsts

import "fmt"

// An ErrorMessage payload is returned from the socketURL if an error occurred
// when connecting.
type ErrorMessage struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error returns a string presentation of the ErrorMessage satifying the error
// interface.
func (e *ErrorMessage) Error() string {
	return fmt.Sprintf("%sError: %s", e.Code, e.Message)
}

// A DisplayEntry list is returned from the socketURL when listing displays.
// Any request that doesn't try to upgrade the request to a websocket will get
// a list of displays as response.
type DisplayEntry struct {
	Display     string `json:"display"`
	Description string `json:"description"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
}
