package dockerengine

import (
	"context"
	"strings"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
	"github.com/taskcluster/taskcluster-worker/runtime/caching"
	"github.com/taskcluster/taskcluster-worker/runtime/gc"
)

const dockerPullImageInactivityTimeout = 30 * time.Second

type imageResource struct {
	Repository string
	Tag        string
	Size       uint64
	client     *docker.Client
	monitor    runtime.Monitor
	dispose    atomics.Once
}

type cacheContext struct {
	context.Context
}

func newCacheContext(ctx context.Context) *cacheContext {
	if ctx != nil {
		return &cacheContext{
			Context: ctx,
		}
	}
	return nil
}

func (ctx *cacheContext) Progress(description string, percent float64) {
	debug("progress: %s %f", description, percent)
}

func (im *imageResource) MemorySize() (uint64, error) {
	return im.Size, nil
}

func (im *imageResource) DiskSize() (uint64, error) {
	return 0, gc.ErrDisposableSizeNotSupported
}

func (im *imageResource) Dispose() error {
	var err error
	im.dispose.Do(func() {
		err = im.client.RemoveImage(buildImageName(im.Repository, im.Tag))
	})
	im.dispose.Wait()
	if err != nil {
		debug("error %v")
		im.monitor.ReportError(err, "error removing image")
		return runtime.ErrNonFatalInternalError
	}
	return nil
}

func imageConstructor(ctx caching.Context, opts interface{}) (caching.Resource, error) {
	options := opts.(imageType)
	client := options.engine.client
	monitor := options.engine.monitor.WithPrefix("image-cache").WithTag("image", buildImageName(options.Repository, options.Tag))

	// TODO: Use outputstream to write progress
	debug("pulling image %s %s", options.Repository, options.Tag)
	err := client.PullImage(docker.PullImageOptions{
		Repository:        options.Repository,
		Tag:               options.Tag,
		InactivityTimeout: dockerPullImageInactivityTimeout,
		Context:           ctx,
	}, docker.AuthConfiguration{})

	if err != nil {
		debug("error pulling image %v", err)
		return nil, err
	}
	// Inspect image to find size
	image, err := client.InspectImage(buildImageName(options.Repository, options.Tag))
	if err != nil {
		monitor.ReportError(err, "error inspecting image")
		debug("error inspecting image %v")
	}
	size := uint64(0)
	if image != nil {
		size = uint64(image.Size)
	}
	debug("image size: %d", size)
	return &imageResource{
		Repository: options.Repository,
		Tag:        options.Tag,
		Size:       size,
		client:     client,
		monitor:    monitor,
	}, nil
}

func buildImageName(repository, tag string) string {
	if strings.HasPrefix(tag, "sha256:") {
		return repository + "@" + tag
	}
	return repository + ":" + tag
}
