package imagecache

import (
	docker "github.com/fsouza/go-dockerclient"
	"github.com/pkg/errors"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
	"github.com/taskcluster/taskcluster-worker/runtime/gc"
)

type image struct {
	ImageName  string // Docker image name to be referenced when creating containers
	size       int64
	docker     *docker.Client
	monitor    runtime.Monitor
	disposed   atomics.Once
	disposeErr error
}

func newImage(imageName string, docker *docker.Client, monitor runtime.Monitor) (*image, error) {
	img, err := docker.InspectImage(imageName)
	if err != nil {
		return nil, errors.Wrapf(err, "docker.InspectImage('%s') when image have just be created", imageName)
	}
	return &image{
		ImageName: imageName,
		size:      img.Size,
		docker:    docker,
		monitor:   monitor,
	}, nil
}

func (i *image) MemorySize() (uint64, error) {
	return 0, gc.ErrDisposableSizeNotSupported
}

func (i *image) DiskSize() (uint64, error) {
	if i.size <= 0 {
		return 0, gc.ErrDisposableSizeNotSupported
	}
	return uint64(i.size), nil
}

func (i *image) Dispose() error {
	i.disposed.Do(func() {
		debug("disposing image: %s", i.ImageName)
		err := i.docker.RemoveImage(i.ImageName)
		if err != nil {
			i.disposeErr = runtime.ErrNonFatalInternalError
			i.monitor.ReportError(err, "docker.RemoveImage failed to remove image")
		}
	})
	i.disposed.Wait()
	return i.disposeErr
}
