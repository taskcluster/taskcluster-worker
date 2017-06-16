package fetcher

import "fmt"

// BrokenReferenceError is used to communicate references that are broken.
// Error message shall be some human-readable explanation of why fetching failed
// and is expected to fail consistently.
type BrokenReferenceError struct {
	subject string // thing we failed to fetch
	reason  string // reason we failed
}

func newBrokenReferenceError(subject, reason string) BrokenReferenceError {
	return BrokenReferenceError{
		subject: subject,
		reason:  reason,
	}
}

func (e BrokenReferenceError) Error() string {
	return fmt.Sprintf("failed to fetch %s, %s", e.subject, e.reason)
}

// IsBrokenReferenceError returns true, if err is a BrokenReferenceError error.
//
// This auxiliary function helps ensure that we type cast correctly.
func IsBrokenReferenceError(err error) bool {
	_, ok := err.(BrokenReferenceError)
	return ok
}
