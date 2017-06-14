package qemuengine

import (
	"fmt"

	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/fetcher"
)

// A fetcher for downloading images.
var imageFetcher = fetcher.Combine(
	// Allow fetching images from URL
	fetcher.URL,
	// Allow fetching images from queue artifacts
	fetcher.Artifact,
)

type fetchImageContext struct {
	*runtime.TaskContext
}

func (c fetchImageContext) Progress(description string, percent float64) {
	c.Log(fmt.Sprintf("Fetching image: %s - %f %%", description, percent))
}
