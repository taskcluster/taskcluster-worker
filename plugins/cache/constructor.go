package cache

import (
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/pkg/errors"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/caching"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
	"github.com/taskcluster/taskcluster-worker/runtime/fetcher"
)

type cacheOptions struct {
	Name               string               `json:"name"`
	Options            interface{}          `json:"options"`
	Preload            interface{}          `json:"preload"`
	ReferenceHash      string               `json:"referenceHash"`
	InitialTaskContext *runtime.TaskContext `json:"-"`
	Reference          fetcher.Reference    `json:"-"`
	Plugin             *plugin              `json:"-"`
}

type preloadFetchContext struct {
	caching.Context
	InitialTaskContext *runtime.TaskContext
	rootURL            *url.URL
}

func (c *preloadFetchContext) Queue() client.Queue {
	return c.InitialTaskContext.Queue()
}

func (c *preloadFetchContext) RootURL() *url.URL {
	return c.rootURL
}

type progressContext struct {
	*runtime.TaskContext
	Name    string
	rootURL *url.URL
}

func (c *progressContext) Progress(description string, percent float64) {
	if c.Name == "" {
		c.Log(fmt.Sprintf("Fetching cache preload from: %s - %.0f %%", description, percent*100))
	} else {
		c.Log(fmt.Sprintf("Fetching cache preload for %s from: %s - %.0f %%", c.Name, description, percent*100))
	}
}

func (c *progressContext) RootURL() *url.URL {
	return c.rootURL
}

func constructor(ctx caching.Context, opts interface{}) (caching.Resource, error) {
	options := opts.(cacheOptions) // must of this type

	// Define the created timestamp (we do this first to ensure volumes get purged if older)
	created := time.Now()

	// Log it when we create named (mutable caches)
	if options.Name != "" {
		options.Plugin.monitor.Infof("creating cache: '%s'", options.Name)
	}

	// If there is no reference, we have nothing to fetch and this is easy
	if options.Reference == nil {
		// Create a new volume
		volume, err := options.Plugin.engine.NewVolume(options.Options)
		if err != nil {
			if err == engines.ErrFeatureNotSupported {
				return nil, runtime.NewMalformedPayloadError(
					"worker engine doesn't support cache volumes",
				)
			}
			return nil, errors.Wrap(err, "failed to create cache volume, engine error")
		}

		return &cacheVolume{
			Volume:  volume,
			Name:    options.Name,
			Created: created,
		}, nil
	}
	// the rest of this function deals with creating a pre-loaded cache

	// Fetch pre-load data to temporary file
	file, err := options.Plugin.environment.TemporaryStorage.NewFile()
	if err != nil {
		return nil, errors.Wrap(err, "unable to create temporary file to fetch cache pre-load")
	}
	defer file.Close() // remove the temporary file whatever happens
	err = options.Reference.Fetch(&preloadFetchContext{
		Context:            ctx,
		InitialTaskContext: options.InitialTaskContext,
	}, &fetcher.FileReseter{File: file})
	if err != nil {
		if fetcher.IsBrokenReferenceError(err) {
			err = runtime.NewMalformedPayloadError(fmt.Sprintf(
				"cache pre-loading error: %s", err.Error(),
			))
		} else {
			err = errors.Wrap(err, "failed to fetch cache preload data")
		}
		return nil, err
	}

	// Seek to start of file (after download)
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		incidentID := options.Plugin.monitor.ReportError(err, "failed to seek to start of temporary file after download")
		ctx.Progress(fmt.Sprintf("internal error downloading, incidentId: %s", incidentID), 1)
		return nil, runtime.ErrFatalInternalError // if we can't seek start that's pretty critical
	}

	// Create a new volume builder
	volumeBuilder, err := options.Plugin.engine.NewVolumeBuilder(options.Options)
	if err != nil {
		if err == engines.ErrFeatureNotSupported {
			return nil, runtime.NewMalformedPayloadError(
				"worker engine doesn't support pre-loaded cache volumes",
			)
		}
		return nil, errors.Wrap(err, "failed to create VolumeBuilder for a pre-loaded cache")
	}

	// Extract the pre-load archive
	if err = extractArchive(file, volumeBuilder); err != nil {
		if verr := volumeBuilder.Discard(); verr != nil {
			options.Plugin.monitor.ReportError(verr, "VolumeBuilder.Discard() failed, after failed archive extraction")
		}
		return nil, err
	}

	// Build the volume
	volume, err := volumeBuilder.BuildVolume()
	if err != nil {
		return nil, errors.Wrap(err, "VolumeBuilder.BuildVolume() failed")
	}

	return &cacheVolume{
		Volume:  volume,
		Name:    options.Name,
		Created: created,
	}, nil
}
