package dockerengine

import (
	"archive/tar"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
	"io"
	"time"
)

type resultSet struct {
	engines.ResultSetBase
	success     bool
	containerID string
	client      *docker.Client
	tempStorage runtime.TemporaryStorage
}

func newResultSet(success bool, containerID string, client *docker.Client, ts runtime.TemporaryStorage) *resultSet {
	return &resultSet{
		success:     success,
		containerID: containerID,
		client:      client,
		tempStorage: ts,
	}
}

func (r *resultSet) Success() bool {
	return r.success
}

func (r *resultSet) ExtractFile(path string) (ioext.ReadSeekCloser, error) {
	// Use DownloadFromContainer to get the tar archive of the required
	// file/folder and unzip.
	tarFile, err := r.extractFromContainer(path)
	if err != nil {
		return nil, engines.ErrResourceNotFound
	}

	defer func() {
		_ = tarFile.Close()
	}()
	tempFile, err := r.tempStorage.NewFile()
	if err != nil {
		return nil, engines.ErrResourceNotFound
	}
	reader := tar.NewReader(tarFile)
	_, err = io.Copy(tempFile, reader)
	if err != nil {
		return nil, engines.ErrResourceNotFound
	}
	return tempFile, nil
}

func (r *resultSet) ExtractFolder(path string, handler engines.FileHandler) error {
	tarFile, err := r.extractFromContainer(path)
	if err != nil {
		return engines.ErrResourceNotFound
	}

	defer func() {
		_ = tarFile.Close()
	}()

	reader := tar.NewReader(tarFile)
	// Instead of using runtime.Untar it seems simpler
	// to unpack each file one at a time and pass it to
	// the handler.
	for {
		hdr, err := reader.Next()
		if err != nil {
			break
		}

		tempFile, err := r.tempStorage.NewFile()
		if err != nil {
			return engines.ErrResourceNotFound
		}
		if _, err = io.Copy(tempFile, reader); err != nil {
			return engines.ErrResourceNotFound
		}

		defer func() {
			_ = tempFile.Close()
		}()

		if handler(hdr.Name, tempFile) != nil {
			return engines.ErrHandlerInterrupt
		}
	}
	return nil
}

func (r *resultSet) Dispose() error {
	if r.tempStorage != nil {
		_ = r.tempStorage.(runtime.TemporaryFolder).Remove()
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
	tempFile, err := r.tempStorage.NewFile()
	if err != nil {
		return nil, err
	}

	opts := docker.DownloadFromContainerOptions{
		OutputStream:      tempFile,
		Path:              path,
		InactivityTimeout: 5 * time.Second,
	}

	err = r.client.DownloadFromContainer(r.containerID, opts)
	if err != nil {
		_ = tempFile.Close()
		return nil, err
	}
	return tempFile, nil
}
