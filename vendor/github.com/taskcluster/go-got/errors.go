package got

import (
	"errors"
	"fmt"
)

// BadResponseCodeError is used to indicate a non-2xx status code
type BadResponseCodeError struct {
	*Response
}

func (e BadResponseCodeError) Error() string {
	return fmt.Sprintf(
		"Non-2xx StatusCode: %d received in %d attempts",
		e.StatusCode, e.Attempts,
	)
}

// ErrResponseTooLarge is used to indicate that the response was larger than
// the safety limit at MaxSize (defined the request)
var ErrResponseTooLarge = errors.New("Response body is larger than MaxSize")
