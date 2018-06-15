// +build linux

package imagecache

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/DataDog/zstd"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/pkg/errors"
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/caching"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
	"github.com/taskcluster/taskcluster-worker/runtime/fetcher"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

// ImageCache wraps caching.Cache such that we don't need to do any casting
// inside the engine and sandbox implementations
type ImageCache struct {
	cache   *caching.Cache
	docker  *docker.Client
	monitor runtime.Monitor
	env     *runtime.Environment
}

// New creates a new ImageCache object
func New(d *docker.Client, env *runtime.Environment, monitor runtime.Monitor) *ImageCache {
	ic := &ImageCache{
		docker:  d,
		monitor: monitor,
		env:     env,
	}
	ic.cache = caching.New(ic.constructor, true, env.GarbageCollector, monitor)
	return ic
}

var imageFetcher = fetcher.Combine(
	fetcher.URLHash,
	fetcher.Index,
	fetcher.Artifact,
)

var imagePullSchema = schematypes.String{
	Title: "Pull Image from Registry",
}

// ImageSchema returns the JSON schema by which images can be specified when
// ImageCache.Require is called
func (ic *ImageCache) ImageSchema() schematypes.Schema {
	// If imageFetcher.Schema() is a oneOf, we avoid wrapping in an extra oneOf...
	// This should always be the same, but by gracefully falling back to the naive implementation
	// things will be okay, even if the fetcher.Combine() internal implementation changes.
	if oneOf, ok := imageFetcher.Schema().(schematypes.OneOf); ok {
		return append(schematypes.OneOf{imagePullSchema}, oneOf...)
	}
	return schematypes.OneOf{
		imageFetcher.Schema(),
		imagePullSchema,
	}
}

// imageOptions is the struct we pass to caching.Cache.Require() which then
// passes it to imageCache.constructor as opts
type imageOptions struct {
	Image     string              `json:"image,omitempty"`
	HashKey   string              `json:"hashKey,omitempty"` // hash of resolved reference
	reference fetcher.Reference   // present so we can fetch resolved reference
	queue     func() client.Queue // present so we fetch resolved reference
}

func (ic *ImageCache) constructor(ctx caching.Context, opts interface{}) (caching.Resource, error) {
	options := opts.(imageOptions) // this is called by Require which is always passed imageOptions

	// Pull from docker
	if options.reference == nil {
		return ic.dockerPullFromRegistry(ctx, options.Image)
	}

	// Load from reference
	return ic.dockerLoadFromReference(cachingContextWithQueue{ctx, options.queue, ic.env.RootURL}, options.reference)
}

// ImageHandle wraps caching.Handle such that we don't need to do any casting
// to access the ImageName
type ImageHandle struct {
	*caching.Handle
	ImageName string
}

// Require that an image is loaded or returned from cache. The imagePayload given
// must satisfy ImageCache.ImageSchema()
func (ic *ImageCache) Require(ctx *runtime.TaskContext, imagePayload interface{}) (*ImageHandle, error) {
	// Caller must ensure that imagePayload matches the schema
	schematypes.MustValidate(ic.ImageSchema(), imagePayload)

	// create image options for the constructor
	// the options structure will contain the data necessary to create the image
	// and if the properties are JSON serialized they can be used to hash the object
	var options imageOptions
	var prefix string
	if s, ok := imagePayload.(string); ok {
		prefix = "Pulling image"
		options.Image = s
	} else {
		prefix = "Fetching image"
		ref, err := imageFetcher.NewReference(taskContextWithProgress{ctx, "Resolving image reference", ic.env.RootURL}, imagePayload)
		if err != nil {
			return nil, runtime.NewMalformedPayloadError(fmt.Sprintf(
				"unable to resolve docker image reference error: %s", err.Error(),
			))
		}
		scopeSets := ref.Scopes()
		if !ctx.HasScopes(scopeSets...) {
			// Construct a neat array of strings "'<scope1>', '<scope2>'" for each scope-set
			sets := []string{}
			for _, scopeSet := range scopeSets {
				sets = append(sets, "'"+strings.Join(scopeSet, "', '")+"'")
			}
			return nil, runtime.NewMalformedPayloadError(fmt.Sprintf(
				"insufficient task.scopes to access referenced docker image, which requires: %s",
				"\n * "+strings.Join(sets, ", or\n * "),
			))
		}
		options.reference = ref
		options.HashKey = ref.HashKey()
		options.queue = ctx.Queue
	}

	handle, err := ic.cache.Require(taskContextWithProgress{ctx, prefix, ic.env.RootURL}, options)
	if err != nil {
		return nil, err
	}
	return &ImageHandle{
		handle,
		handle.Resource().(*image).ImageName,
	}, nil
}

const dockerPullImageInactivityTimeout = 5 * 60 * time.Second

// dockerPullFromRegistry creates an image resources by pulling from a docker registry
func (ic *ImageCache) dockerPullFromRegistry(ctx caching.Context, imageName string) (*image, error) {
	r, w := io.Pipe()
	var err error
	util.Parallel(func() {
		defer w.Close() // always close the writer
		// Try to pull the image
		err = ic.docker.PullImage(docker.PullImageOptions{
			Context:           ctx,
			Repository:        imageName,
			InactivityTimeout: dockerPullImageInactivityTimeout,
			OutputStream:      w,
			RawJSONStream:     true,
		}, docker.AuthConfiguration{})
	}, func() {
		reportDockerPullProgress(ctx, r)
		r.Close()
	})
	if err != nil {
		if e, ok := err.(*docker.Error); ok && e.Status == 404 {
			return nil, runtime.NewMalformedPayloadError(fmt.Sprintf(
				"failed to pull docker image '%s' is missing or authentication is required, error: %s",
				imageName, e.Message,
			))
		}
		return nil, errors.Wrapf(err, "failed to pull image: %s", imageName)
	}
	return newImage(imageName, ic.docker, ic.monitor.WithTag("pulled-image", imageName))
}

// dockerLoadFromReference will download image zstd compressed tar-ball from reference
// decompress and rename it on-the-fly using FetchAsStream to support fetching retries
// without hitting disk before loading it into docker.
func (ic *ImageCache) dockerLoadFromReference(ctx fetcher.Context, reference fetcher.Reference) (caching.Resource, error) {
	// Docker images names must be lower case, for security this should be unpredictable
	imageName := "fetched-image/" + strings.ToLower(slugid.Nice())
	err := fetcher.FetchAsStream(ctx, reference, func(ctx context.Context, r io.Reader) error {
		// Create zstd reader for the compressed tar-stream
		zr := zstd.NewReader(r)
		defer zr.Close() // cleanup resources (frees underlying zstd C resources)

		// Create errors for rename of tar-stream and docker load of image
		var rerr, derr error
		ir, iw := io.Pipe() // create an image pipe for the renamed image

		// ensure we cancel the docker load call
		c, cancel := context.WithCancel(ctx)
		defer cancel()
		util.Parallel(func() {
			defer func() {
				iw.CloseWithError(rerr) // always close the image writer pipe
				if rerr != nil {
					cancel() // cancel early if there was an error
				}
			}()
			rerr = renameDockerImageTarStream(imageName, zr, iw)
		}, func() {
			defer func() {
				ir.CloseWithError(derr) // ensure we don't deadlock the write side in case of error
			}()
			derr = ic.docker.LoadImage(docker.LoadImageOptions{
				Context:     c,
				InputStream: ir,
			})
		})
		// if we had an error renaming the docker image, we return it, such an error
		// is always more interesting than any error from loading it with docker.
		if rerr != nil {
			return rerr
		}
		if derr != nil {
			// Presumably any 4xx error is some user error, and we can probably show the message
			if e, ok := derr.(*docker.Error); ok && 400 <= e.Status && e.Status < 500 {
				return runtime.NewMalformedPayloadError(fmt.Sprintf(
					"invalid docker image tar-ball: %s", e.Message,
				))
			}
			return errors.Wrap(derr, "failed to load docker image from tar-ball")
		}
		return nil
	})
	if err != nil {
		// A broken reference is a necessarily a malformed task payload
		if fetcher.IsBrokenReferenceError(err) {
			return nil, runtime.NewMalformedPayloadError("docker image reference is invalid, ", err.Error())
		}
		if _, ok := runtime.IsMalformedPayloadError(err); ok {
			return nil, err
		}
		// This is an internal error or intermittent network error
		return nil, errors.Wrap(err, "failed to fetch docker image from reference")
	}

	// return image resource
	return newImage(imageName, ic.docker, ic.monitor.WithTag("image-fetched", reference.HashKey()))
}
