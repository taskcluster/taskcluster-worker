package fetcher

import (
	"context"
	"net/url"
	"time"

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
	RootURL() *url.URL
}

type contextWithCancel struct {
	context.Context
	parent  Context
	rootURL *url.URL
}

func (c *contextWithCancel) Queue() client.Queue {
	return c.parent.Queue()
}

func (c *contextWithCancel) Progress(description string, percent float64) {
	c.parent.Progress(description, percent)
}

func (c *contextWithCancel) RootURL() *url.URL {
	return c.rootURL
}

// WithCancel returns a Context and a cancel function similar to context.WithCancel
func WithCancel(ctx Context, rootURL *url.URL) (Context, func()) {
	child, cancel := context.WithCancel(ctx)
	return &contextWithCancel{child, ctx, rootURL}, cancel
}
