// +build linux

package imagecache

import (
	"fmt"
	"net/url"

	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/caching"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
)

// cachingContextWithQueue wraps a caching.Context and queue creation function
// to match the interface of fetcher.Context
type cachingContextWithQueue struct {
	caching.Context
	queue   func() client.Queue
	rootURL *url.URL
}

func (c cachingContextWithQueue) Queue() client.Queue {
	return c.queue()
}

func (c cachingContextWithQueue) RootURL() *url.URL {
	return c.rootURL
}

// taskContextWithProgress wraps TaskContext to satisfy the caching.Context
// interface, by adding a Progress() function
type taskContextWithProgress struct {
	*runtime.TaskContext
	Prefix  string
	rootURL *url.URL
}

func (c taskContextWithProgress) Progress(description string, percent float64) {
	c.Log(fmt.Sprintf("%s: %s %.0f %%", c.Prefix, description, percent*100))
}

func (c taskContextWithProgress) RootURL() *url.URL {
	return c.rootURL
}
