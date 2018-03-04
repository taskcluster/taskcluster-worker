package dockerengine

import (
	"archive/tar"
	"io"
	"path/filepath"
	"strings"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/caching"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

type resultSet struct {
	engines.ResultSetBase
	success     bool
	containerID string
	client      *docker.Client
	monitor     runtime.Monitor
	tempStorage runtime.TemporaryStorage
	handle      *caching.Handle
}

func newResultSet(success bool, containerID string, client *docker.Client,
	ts runtime.TemporaryStorage, handle *caching.Handle, monitor runtime.Monitor) *resultSet {
	return &resultSet{
		success:     success,
		containerID: containerID,
		client:      client,
		tempStorage: ts,
		handle:      handle,
		monitor:     monitor,
	}
}

func (r *resultSet) Success() bool {
	return r.success
}

func (r *resultSet) ExtractFile(path string) (ioext.ReadSeekCloser, error) {
	path = filepath.Clean(path)
	monitor := r.monitor.WithTag("extract-file", path)
	// Use DownloadFromContainer to get the tar archive of the required
	// file/folder and unzip.
	tarfile, err := r.extractFromContainer(path)
	if err != nil {
		return nil, engines.ErrResourceNotFound
	}

	defer func() {
		err = tarfile.Close()
		if err != nil {
			monitor.ReportWarning(err, "could not close temporary tar file")
		}
	}()

	_, err = tarfile.Seek(0, 0)
	if err != nil {
		monitor.ReportError(err, "could not seek to start of file")
		return nil, runtime.ErrNonFatalInternalError
	}

	reader := tar.NewReader(tarfile)
	_, err = reader.Next()
	if err != nil {
		return nil, runtime.ErrNonFatalInternalError
	}

	tempfile, err := r.tempStorage.NewFile()
	if err != nil {
		monitor.ReportError(err, "could not create temporary file")
		return nil, runtime.ErrNonFatalInternalError
	}
	_, err = io.Copy(tempfile, reader)
	if err != nil {
		monitor.ReportError(err, "could not untar extracted file")
		return nil, runtime.ErrNonFatalInternalError
	}

	_, err = tempfile.Seek(0, 0)
	if err != nil {
		monitor.ReportError(err, "could not seek to start of file")
		return nil, runtime.ErrNonFatalInternalError
	}

	return tempfile, nil
}

func (r *resultSet) ExtractFolder(path string, handler engines.FileHandler) error {
	path = filepath.Clean(path)
	monitor := r.monitor.WithTag("extract-folder", path)

	tarfile, err := r.extractFromContainer(path)
	if err != nil {
		return engines.ErrResourceNotFound
	}

	defer func() {
		err = tarfile.Close()
		if err != nil {
			monitor.ReportWarning(err, "could not close temporary file")
		}
	}()

	strip := filepath.Base(path) + "/"
	_, err = tarfile.Seek(0, 0)
	if err != nil {
		monitor.ReportError(err, "could not seek to start of tar file")
		return runtime.ErrNonFatalInternalError
	}
	reader := tar.NewReader(tarfile)
	// Instead of using runtime.Untar it seems simpler
	// to unpack each file one at a time and pass it to
	// the handler.
	for {
		hdr, err := reader.Next()
		if err != nil {
			break
		}
		if hdr.Typeflag == tar.TypeDir {
			continue
		}
		fname := strings.TrimPrefix(hdr.Name, strip)
		m := monitor.WithTag("filename", fname)

		tempfile, err := r.tempStorage.NewFile()
		if err != nil {
			m.ReportError(err, "could not create temporary file")
			continue
		}

		if _, err = io.Copy(tempfile, reader); err != nil {
			m.ReportError(err, "could not copy file")
			continue
		}

		_, err = tempfile.Seek(0, 0)
		if err != nil {
			m.ReportWarning(err, "could not seek to start of file")
			continue
		}
		if handler(fname, tempfile) != nil {
			return engines.ErrHandlerInterrupt
		}

		err = tempfile.Close()
		if err != nil {
			m.ReportWarning(err, "could not close temporary file")
		}
	}
	return nil
}

func (r *resultSet) Dispose() error {
	if r.tempStorage != nil {
		err := r.tempStorage.(runtime.TemporaryFolder).Remove()
		if err != nil {
			r.monitor.ReportWarning(err, "could not remove temporary storage")
		}
	}
	if r.handle != nil {
		r.handle.Release()
	}
	return r.client.RemoveContainer(docker.RemoveContainerOptions{
		ID:    r.containerID,
		Force: true,
	})
}

func (r *resultSet) extractFromContainer(path string) (runtime.TemporaryFile, error) {
	if r.tempStorage == nil {
		return nil, engines.ErrResourceNotFound
	}
	tempfile, err := r.tempStorage.NewFile()
	if err != nil {
		return nil, runtime.ErrNonFatalInternalError
	}

	opts := docker.DownloadFromContainerOptions{
		OutputStream:      tempfile,
		Path:              path,
		InactivityTimeout: 5 * time.Second,
	}

	err = r.client.DownloadFromContainer(r.containerID, opts)
	if err != nil {
		_ = tempfile.Close()
		return nil, engines.ErrResourceNotFound
	}
	return tempfile, nil
}
