package qemuengine

import (
	"fmt"
	"os"

	"github.com/taskcluster/taskcluster-worker/engines/qemu/image"
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

func imageDownloader(c *runtime.TaskContext, image interface{}) image.Downloader {
	return func(imageFile string) error {
		target, err := os.Create(imageFile)
		if err != nil {
			return err
		}
		err = imageFetcher.Fetch(fetchImageContext{c}, image, &fetcher.FileReseter{File: target})
		if err != nil {
			defer target.Close()
			return err
		}
		return target.Close()
	}
}
