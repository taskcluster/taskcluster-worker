package fetcher

import (
	"io"

	schematypes "github.com/taskcluster/go-schematypes"
)

// WriteReseter is a io.Writer with Reset()
// method that discards everything written and starts over from scratch.
//
// This is easily implemented by wrapping os.File with FileReseter.
type WriteReseter interface {
	io.Writer
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
	Fetch(context Context, target WriteReseter) error
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
