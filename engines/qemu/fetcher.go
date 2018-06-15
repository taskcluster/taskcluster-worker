package qemuengine

import (
	"fmt"
	"net/url"

	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/fetcher"
)

// A fetcher for downloading images.
var imageFetcher = fetcher.Combine(
	// Allow fetching images from URL
	fetcher.URL,
	// Allow fetching images from queue artifacts
	fetcher.Artifact,
	// Allow fetching images from queue referenced by index namespace
	fetcher.Index,
	// Allow fetching images from URL + hash
	fetcher.URLHash,
)

type fetchImageContext struct {
	*runtime.TaskContext
	rootURL *url.URL
}

func (c fetchImageContext) Progress(description string, percent float64) {
	c.Log(fmt.Sprintf("Fetching image: %s - %.0f %%", description, percent*100))
}

func (c fetchImageContext) RootURL() *url.URL {
	return c.rootURL
}
