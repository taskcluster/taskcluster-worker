// +build linux

package imagecache

import (
	"fmt"

	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/caching"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
)

// cachingContextWithQueue wraps a caching.Context and queue creation function
// to match the interface of fetcher.Context
type cachingContextWithQueue struct {
	caching.Context
	queue func() client.Queue
}

func (c cachingContextWithQueue) Queue() client.Queue {
	return c.queue()
}

// taskContextWithProgress wraps TaskContext to satisfy the caching.Context
// interface, by adding a Progress() function
type taskContextWithProgress struct {
	*runtime.TaskContext
	Prefix string
}

func (c taskContextWithProgress) Progress(description string, percent float64) {
	c.Log(fmt.Sprintf("%s: %s %.0f %%", c.Prefix, description, percent*100))
}
