package dockerengine

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/pkg/errors"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/docker/network"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

// maxExtractedFileSizeAllowed is the maximum file size that will be read out
// of docker with ExtractFile/ExtraFolder, this is limited for sanity.
const maxExtractedFileSizeAllowed = 256 // GiB

// maxConcurrentFileHandlerCalls is the maximum number of concurrent calls to
// FileHandler in ExtractFolder, this is just a sanity limit.
//
// This limit ensures that we don't create too many temporary files at the
// same time. Thus, alllowing an artifact plugin to abort the ExtractFolder
// call if too many artifacts are extracted.
const maxConcurrentFileHandlerCalls = 5

type resultSet struct {
	engines.ResultSetBase
	success       bool
	containerID   string
	docker        *docker.Client
	monitor       runtime.Monitor
	storage       runtime.TemporaryFolder
	context       *runtime.TaskContext
	networkHandle *network.Handle
}

func (r *resultSet) Success() bool {
	return r.success
}

func (r *resultSet) ExtractFile(path string) (ioext.ReadSeekCloser, error) {
	// We'll treat paths ending with a slash as paths to folders
	if strings.HasSuffix(path, "/") {
		debug("ExtractFile(%s) ends with '/' which can't be a file", path)
		return nil, runtime.NewMalformedPayloadError(fmt.Sprintf(
			"docker file path: '%s' ends with slash, paths to files cannot end with slash", path,
		))
	}

	var result ioext.ReadSeekCloser
	var m sync.Mutex
	err := r.extractResource(path, func(p string, stream ioext.ReadSeekCloser) error {
		m.Lock()
		defer m.Unlock()
		if result != nil {
			stream.Close() // this only happens if 'path' points at a folder, which shouldn't be
			result.Close() // possible because we enforce that it must not end with slash.
			result = nil   // If it happens we discard results, and return an error
			// Ensure that extractResource returns ErrHandlerInterrupt
			return errors.New("abort extracting artifacts")
		}
		result = stream
		return nil
	})

	if err == engines.ErrHandlerInterrupt {
		// docker assumes we're looking for a file when path doesn't end with '/'
		return nil, fmt.Errorf("docker returned multiple files for path: '%s', this is a violation of docker API", path)
	}
	// If we had an error, we just return it
	if err != nil {
		if result != nil {
			result.Close()
		}
		return nil, err
	}
	// If we had no error, and didn't get any response, we assume it's not found
	if result == nil {
		return nil, engines.ErrResourceNotFound
	}
	return result, nil
}

func (r *resultSet) ExtractFolder(path string, handler engines.FileHandler) error {
	// We'll treat paths ending with a slash as paths to folders
	if strings.HasSuffix(path, "/") {
		debug("ExtractFolder(%s) ends with '/'", path)
		return runtime.NewMalformedPayloadError(fmt.Sprintf(
			"docker folder path: '%s' ends with slash, paths to folders must not end with a slash", path,
		))
	}
	path += "/"
	return r.extractResource(path, func(name string, stream ioext.ReadSeekCloser) error {
		// Make the name relative to path
		if !strings.HasPrefix(name, "/") {
			name = "/" + name // Ensure we always have an absolute path
		}
		return handler(name, stream)
	})
}

func (r *resultSet) extractResource(path string, handler engines.FileHandler) error {
	// We force path to be absolute, this is the only sane thing
	if !strings.HasPrefix(path, "/") {
		debug("extractResource(%s) doesn't start with '/', hence it is a relative path", path)
		return runtime.NewMalformedPayloadError(fmt.Sprintf(
			"docker path: '%s' is a relative path, paths must be absolute", path,
		))
	}

	// Read tar stream while we download from docker
	stream, streamWriter := io.Pipe()

	ctx, cancel := context.WithCancel(r.context)
	notFound := false
	var derr, rerr error
	var interrupted atomics.Once
	util.Parallel(func() {
		var wg atomics.WaitGroup
		defer wg.Wait()      // Wait for all handlers to be done
		defer cancel()       // abort DownloadFromContainer when we stop reading the tar-stream
		defer stream.Close() // always close the reader, so writers abort instead of hanging
		// Create tar reader
		reader := tar.NewReader(stream)
		for !interrupted.IsDone() {
			// Get first entry from reader
			var hdr *tar.Header
			hdr, rerr = reader.Next()
			if rerr == io.EOF {
				rerr = nil // EOF is not an error
				return
			}
			if rerr != nil {
				rerr = errors.Wrap(rerr, "failed to read tar-stream from docker")
				return
			}
			if !ioext.IsPlainFileInfo(hdr.FileInfo()) {
				continue // skip entries that aren't files
			}
			// Sanity check on the file size mostly in case someone tries to export
			// a sparse file that contains a lot of zeros.
			if hdr.Size > maxExtractedFileSizeAllowed*1024*1024*1024 {
				rerr = fmt.Errorf(
					"attempted to extract file: '%s' at size: %d larger than allowed %d GiB",
					path, hdr.Size, maxExtractedFileSizeAllowed,
				)
				r.context.LogError(fmt.Sprintf(
					"A plugin attempted to read the file: '%s' which has a size: %d greater than %d GiB, "+
						"which is the maximum allowed file size hardcoded into the worker for sanity",
					path, hdr.Size, maxExtractedFileSizeAllowed,
				))
				return
			}
			debug("extractResource(%s) found file: '%s' of size: %d bytes", path, hdr.Name, hdr.Size)

			// Create temporary file that we extract this to
			var tmpfile runtime.TemporaryFile
			tmpfile, rerr = r.storage.NewFile()
			if rerr != nil {
				rerr = errors.Wrap(rerr, "failed to create temporary file for extracting files from docker")
				return
			}

			// Copy data to tmpfile
			if _, rerr = io.Copy(tmpfile, reader); rerr != nil {
				tmpfile.Close()
				rerr = errors.Wrap(rerr, "failed to create temp-file using tar-stream from docker")
				return
			}

			// Seek start of temp file
			if _, rerr = tmpfile.Seek(0, io.SeekStart); rerr != nil {
				tmpfile.Close()
				rerr = errors.Wrap(rerr, "failed to seek to start of temporary file, after reading from docker")
				return
			}

			// Invoke handler concurrently
			wg.WaitForLessThan(maxConcurrentFileHandlerCalls) // Limit concurrency as a sanity measure
			wg.Add(1)
			go func(t runtime.TemporaryFile) {
				defer wg.Done()
				if handler(hdr.Name, t) != nil {
					interrupted.Do(nil)
				}
			}(tmpfile)
		}
	}, func() {
		debug("extractResource(%s) extracting a folder", path)
		derr = r.docker.DownloadFromContainer(r.containerID, docker.DownloadFromContainerOptions{
			Context:           ctx,
			OutputStream:      streamWriter,
			Path:              path,
			InactivityTimeout: 5 * time.Second,
		})
		if derr != nil && derr == ctx.Err() {
			derr = nil // Ignore any error
			streamWriter.CloseWithError(errors.Wrap(ctx.Err(), "download from docker aborted"))
		} else if derr != nil {
			debug("DownloadFromContainer(%s, {Path: %s, ...) => %s", r.containerID, path, derr)
			streamWriter.CloseWithError(derr)
			if e, ok := derr.(*docker.Error); ok && (e.Status == 400 || e.Status == 404) {
				// Note: this could also be container is missing, but that would be an internal
				//   		 error, as we haven't removed it yet. So we assume that can't happen.
				notFound = true
				derr = nil
			} else {
				derr = errors.Wrap(derr, "docker.DownloadFromContainer failed")
			}
		} else {
			streamWriter.Close()
		}
	})

	if interrupted.IsDone() {
		return engines.ErrHandlerInterrupt
	}
	if notFound {
		return engines.ErrResourceNotFound
	}
	if rerr != nil {
		r.monitor.ReportError(rerr, "problem while reading tar-stream from docker")
		return runtime.ErrNonFatalInternalError
	}
	if derr != nil {
		r.monitor.ReportError(derr, "problem getting the tar-stream from docker")
		return runtime.ErrNonFatalInternalError
	}
	return nil
}

func (r *resultSet) Dispose() error {
	hasErr := false

	// Remove the container
	err := r.docker.RemoveContainer(docker.RemoveContainerOptions{
		ID:            r.containerID,
		Force:         true, // Kill anything still running in the container
		RemoveVolumes: true, // Remove any volumes automatically created with the container (VOLUME in docker image)
	})
	if err != nil {
		r.monitor.ReportError(err, "failed to remove container in disposal of resultSet")
		hasErr = true
	}

	// Remove temporary storage
	if err = r.storage.Remove(); err != nil {
		r.monitor.ReportError(err, "failed to remove temporary storage in disposal of resultSet")
		hasErr = true
	}

	// Release the network
	r.networkHandle.Release()

	// If ErrNonFatalInternalError if there was an error of any kind
	if hasErr {
		return runtime.ErrNonFatalInternalError
	}
	return nil
}
