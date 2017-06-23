package fetcher

import (
	"context"
	"io"
	"time"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
)

// Time between progress reports, defined here so it can easily be modified in
// tests where a 5s delay is undesirable.
var progressReportInterval = 5 * time.Second

// Context for fetching resource from a reference.
type Context interface {
	context.Context      // Context for aborting the fetch operation
	Queue() client.Queue // Client with credentials covering Fetcher.Scopes()
	// Print a progress report that looks somewhat like this:
	//     "Fetching <description> - <percent> %"
	// The <percent> is given as a float between 0 and 1, when formatting
	// consumers may wish to round to one decimal using "%.0f" formatting.
	// Progress reports won't be sent more than once every 5 seconds.
	Progress(description string, percent float64)
}

// WriteSeekReseter is a io.Writer + io.Seeker + io.Closer with Reset()
// method that discards everything written and starts over from scratch.
//
// This is easily implemented by wrapping os.File with FileReseter.
type WriteSeekReseter interface {
	io.Writer
	io.Seeker
	Reset() error
}

// A Reference to a blob that can be fetched.
type Reference interface {
	// Unique key for the reference.
	//
	// Useful for caching resources.
	HashKey() string

	// List of scope-sets that could grant access to the reference.
	// Tasks using a cached instance of this resources should satisfy at-least
	// one of these scope-sets.
	//
	// Returns [][]string{[]string{}} if no scopes are required.
	Scopes() [][]string

	// Fetch a reference to a target, sending progress to Context as well
	// as returning a human readable error message, if fetching fails.
	// If the referenced resource doesn't exist it returns a BrokenReferenceError.
	Fetch(context Context, target WriteSeekReseter) error
}

// A Fetcher specifies a schema for references that it knows how to fetch.
// It also provides a method to generate a HashKey for each valid reference,
// as well as a list of scopes required for a task to use a reference.
type Fetcher interface {
	// Schema for references, should **only** match this type
	Schema() schematypes.Schema
	// NewReference returns a reference for options matching Schema.
	//
	// This method may fully or partially resolve the reference, in-order to be
	// able to return a consistent HashKey. Hence, this method may also return
	// a human-readable error message.
	// If the referenced resource doesn't exist it returns a BrokenReferenceError.
	NewReference(context Context, options interface{}) (Reference, error)
}
