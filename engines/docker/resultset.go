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
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
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
	path = filepath.Clean(path)
	// Use DownloadFromContainer to get the tar archive of the required
	// file/folder and unzip.
	tarfile, err := r.extractFromContainer(path)
	if err != nil {
		return nil, engines.ErrResourceNotFound
	}
	defer func() {
		_ = tarfile.Close()
	}()
	tarfile.Seek(0, 0)
	reader := tar.NewReader(tarfile)
	_, err = reader.Next()
	if err != nil {
		return nil, runtime.ErrNonFatalInternalError
	}
	tempfile, err := r.tempStorage.NewFile()
	_, err = io.Copy(tempfile, reader)
	tempfile.Seek(0, 0)
	return tempfile, nil
}

func (r *resultSet) ExtractFolder(path string, handler engines.FileHandler) error {
	path = filepath.Clean(path)
	tarfile, err := r.extractFromContainer(path)
	if err != nil {
		return engines.ErrResourceNotFound
	}

	// defer func() {
	// 	_ = tarfile.Close()
	// }()

	strip := filepath.Base(path) + "/"
	tarfile.Seek(0, 0)
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
		// fmt.Println(hdr)

		tempfile, err := r.tempStorage.NewFile()
		if err != nil {
			return engines.ErrResourceNotFound
		}
		if _, err = io.Copy(tempfile, reader); err != nil {
			return engines.ErrResourceNotFound
		}

		defer func() {
			_ = tempfile.Close()
		}()

		fname := strings.TrimPrefix(hdr.Name, strip)
		tempfile.Seek(0, 0)
		if handler(fname, tempfile) != nil {
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
	// fmt.Println("extracting ", path, " from container storage")
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
	// fmt.Println("downloaded from container to ", tempfile.Path())
	return tempfile, nil
}
