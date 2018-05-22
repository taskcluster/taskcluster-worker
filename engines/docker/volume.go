// +build linux

package dockerengine

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

// dockerVolumeCreateTimeout is the maximum amount of time we allow the docker
// volume creation to take. This ensures that request timeouts doesn't cause the
// worker to hang.
const dockerVolumeCreateTimeout = 5 * time.Minute

// dockerVolumeRemoveTimeout is the maximum amount of time we allow the docker
// volume removal to take. This a bit longer than dockerVolumeCreateTimeout
// in order to give the OS time to remove a lot of files.
const dockerVolumeRemoveTimeout = 25 * time.Minute

var volumeSchema = schematypes.Object{
	Title:                "Docker Volume Options",
	Description:          "Options for docker volumes at this time no options are supported.",
	AdditionalProperties: false, // We just require an empty object to ensure forward-compatibility
}

type volumeOptions struct{}

type volumeBuilder struct {
	engines.VolumeBuilderBase
	m       sync.Mutex
	v       *volume
	invalid bool
}

type volume struct {
	engines.VolumeBase
	m        sync.Mutex
	name     string
	engine   *engine
	options  *volumeOptions
	volume   *docker.Volume
	monitor  runtime.Monitor
	disposed bool
}

func newVolumeBuilder(e *engine, options *volumeOptions) (*volumeBuilder, error) {
	name := slugid.Nice()

	// Create a monitor to follow this volume for it's life-cycle
	monitor := e.monitor.WithTag("volume-name", name)

	// Ensure we have a timeout on the CreateVolume for sanity, this ensures that
	// the entire worker doesn't deadlock if docker has an issue
	context, cancel := context.WithTimeout(context.Background(), dockerVolumeCreateTimeout)
	defer cancel()

	debug("creating volume name: '%s'", name)
	vol, err := e.docker.CreateVolume(docker.CreateVolumeOptions{
		Context: context,
		Name:    name,
		Driver:  "local",
		Labels: map[string]string{
			"created": time.Now().UTC().Format(time.RFC3339),
			"owner":   "taskcluster-worker",
		},
	})
	if err != nil {
		monitor.ReportError(err, "docker.CreateVolume failed")
		return nil, runtime.ErrFatalInternalError
	}

	return &volumeBuilder{
		v: &volume{
			name:    name,
			engine:  e,
			options: options,
			volume:  vol,
			monitor: monitor,
		},
	}, nil
}

func (vb *volumeBuilder) WriteFolder(name string) error {
	vb.m.Lock()
	defer vb.m.Unlock()

	// Ensure that this VolumeBuilder instance is still valid
	if vb.invalid {
		vb.v.monitor.Panic("VolumeBuilder.WriteFolder() was called after BuildVolume()/Discard()")
	}

	// Find path to folder
	p := filepath.Join(vb.v.volume.Mountpoint, filepath.FromSlash(name))

	// Forbid file paths that reaches out-side the docker volume mount-point
	if !strings.HasPrefix(p, filepath.Clean(vb.v.volume.Mountpoint)) {
		vb.v.monitor.WithTags(map[string]string{
			"folderName": name,
			"mountPoint": vb.v.volume.Mountpoint,
		}).ReportError(errors.New(
			"VolumeBuilder.WriteFolder() for docker-engine attempted to create folder outside the volume",
		))
		return runtime.ErrFatalInternalError
	}

	// Create folder
	err := os.MkdirAll(p, 0777)
	if err != nil {
		vb.v.monitor.WithTags(map[string]string{
			"folderName": name,
			"mountPoint": vb.v.volume.Mountpoint,
		}).ReportError(err, "VolumeBuilder.WriteFolder() for docker-engine failed to create folder")
		return runtime.ErrFatalInternalError // we can't really continue if we failed to create a folder
	}

	return nil
}

func (vb *volumeBuilder) WriteFile(name string) io.WriteCloser {
	vb.m.Lock()
	defer vb.m.Unlock()

	// Ensure that this VolumeBuilder instance is still valid
	if vb.invalid {
		vb.v.monitor.Panic("VolumeBuilder.WriteFile() was called after BuildVolume()/Discard()")
	}

	// Find path to file
	p := filepath.Join(vb.v.volume.Mountpoint, filepath.FromSlash(name))
	d := filepath.Clean(filepath.Dir(p)) // folder path that we should ensure exists

	// Forbid file paths that reaches out-side the docker volume mount-point
	if !strings.HasPrefix(d, filepath.Clean(vb.v.volume.Mountpoint)) {
		vb.v.monitor.WithTags(map[string]string{
			"fileName":   name,
			"mountPoint": vb.v.volume.Mountpoint,
		}).ReportError(errors.New(
			"VolumeBuilder.WriteFile() for docker-engine attempted to create file outside the volume",
		))
		return &errWriteCloser{Err: runtime.ErrFatalInternalError}
	}

	// Create all the necessary folders
	err := os.MkdirAll(d, 0777)
	if err != nil {
		vb.v.monitor.WithTags(map[string]string{
			"fileName":   name,
			"mountPoint": vb.v.volume.Mountpoint,
		}).ReportError(err, "VolumeBuilder.WriteFile() for docker-engine failed to necessary folders for file")
		return &errWriteCloser{Err: runtime.ErrFatalInternalError}
	}

	f, err := os.OpenFile(p, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0777)
	if err != nil {
		vb.v.monitor.WithTags(map[string]string{
			"fileName":   name,
			"mountPoint": vb.v.volume.Mountpoint,
		}).ReportError(err, "VolumeBuilder.WriteFile() for docker-engine failed to create file")
		// we can't really continue if we failed to create a file
		return &errWriteCloser{Err: runtime.ErrFatalInternalError}
	}

	return f // Note it is the callers responsibility to close the file
}

func (vb *volumeBuilder) BuildVolume() (engines.Volume, error) {
	vb.m.Lock()
	defer vb.m.Unlock()

	// Ensure that this VolumeBuilder instance is still valid
	if vb.invalid {
		vb.v.monitor.Panic("VolumeBuilder.BuildVolume() was called after BuildVolume()/Discard()")
	}

	// Mark the VolumeBuilder as invalid
	vb.invalid = true

	return vb.v, nil
}

func (vb *volumeBuilder) Discard() error {
	vb.m.Lock()
	defer vb.m.Unlock()

	// Ensure that this VolumeBuilder instance is still valid
	if vb.invalid {
		vb.v.monitor.Panic("VolumeBuilder.Discard() was called after BuildVolume()/Discard()")
	}

	// Mark the VolumeBuilder as invalid
	vb.invalid = true

	// Discard the underlying volume
	return vb.v.Dispose()
}

// GetName returns the name for mounting it in docker containers
func (v *volume) GetName() string {
	v.m.Lock()
	defer v.m.Unlock()

	// Validate that this haven't been disposed yet
	if v.disposed {
		v.monitor.Panic("Volume cannot be used after Dispose()")
	}

	return v.name
}

func (v *volume) Dispose() error {
	v.m.Lock()
	defer v.m.Unlock()

	// Ignore double Dispose() there is no risk here
	if v.disposed {
		// Send a warning regardless
		v.monitor.Warn("Volume.Discard() was called twice!")
		return nil
	}

	// Ensure that we don't dispose twice
	v.disposed = true

	// Ensure we have some timeout, this should be large as there could be many
	// files to delete. Most linux file systems will be fast at this, but no promises.
	context, cancel := context.WithTimeout(context.Background(), dockerVolumeRemoveTimeout)
	defer cancel()

	debug("removing volume name: '%s'", v.name)
	err := v.engine.docker.RemoveVolumeWithOptions(docker.RemoveVolumeOptions{
		Context: context,
		Name:    v.name,
	})
	// If there is an error, we report it and return ErrNonFatalInternalError.
	// (non-fatal because leaking a single volume isn't the end of the world)
	if err != nil {
		v.monitor.ReportError(err, "Volume.Dispose() failed to remove a volume")
		return runtime.ErrNonFatalInternalError
	}

	return nil
}

// errWriteCloser is a simple io.WriteCloser implementation that returns Err
// for all operations.
type errWriteCloser struct {
	Err error
}

func (e *errWriteCloser) Write(p []byte) (int, error) {
	return 0, e.Err
}

func (e *errWriteCloser) Close() error {
	return e.Err
}
