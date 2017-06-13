package fetcher

import "fmt"

// BrokenReferenceError is used to communicated references that are broken.
// Error message shall be some human-readable explanation of why fetching failed
// and is expected to fail consistently.
type BrokenReferenceError struct {
	message string
}

func newBrokenReferenceError(format string, args ...interface{}) BrokenReferenceError {
	return BrokenReferenceError{
		message: fmt.Sprintf(format, args...),
	}
}

func (e BrokenReferenceError) Error() string {
	return e.message
}

// IsBrokenReferenceError returns true, if err is a BrokenReferenceError error.
//
// This auxiliary function helps ensure that we type cast correctly.
func IsBrokenReferenceError(err error) bool {
	_, ok := err.(BrokenReferenceError)
	return ok
}
