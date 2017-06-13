package fetcher

import (
	"context"
	"io"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
)

// Context for fetching resource from a reference.
type Context interface {
	context.Context      // Context for aborting the fetch operation
	Queue() client.Queue // Client with credentials covering Fetcher.Scopes()
	// Print a progress report that looks somewhat like this:
	//     "Fetching <description> - <percent> %"
	// Progress reports won't be sent more than once every 10 seconds.
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

// A Fetcher specifies a schema for references that it knows how to fetch.
// It also provides a method to generate a HashKey for each valid reference,
// as well as a list of scopes required for a task to use a reference.
type Fetcher interface {
	// Schema for references, should **only** match this type
	Schema() schematypes.Schema

	// Unique key for a reference (reference must validate against schema)
	//
	// Useful for caching resources.
	HashKey(reference interface{}) string

	// List of scope-sets that could grant access to the reference.
	// Tasks using a cached instance of this resources should satisfy at-least
	// one of these scope-sets.
	//
	// Returns [][]string{[]string{}} if no scopes are required.
	Scopes(reference interface{}) [][]string

	// Fetch a reference to a target, sending progress to Context as well
	// as returning a human readable error message, if fetching fails.
	// If the referenced resource doesn't exist it returns a BrokenReferenceError.
	Fetch(context Context, reference interface{}, target WriteSeekReseter) error
}
