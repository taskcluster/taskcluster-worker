package cache

import (
	"archive/tar"
	"bufio"
	"fmt"
	"io"

	"github.com/pkg/errors"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"gopkg.in/h2non/filetype.v1"
	"gopkg.in/h2non/filetype.v1/matchers"
)

type fileSystem interface {
	WriteFolder(name string) error
	WriteFile(name string) io.WriteCloser
}

// extractArchive detects archive type from source and extracts to target
// If this fails due to archive format then it returns MalformedPayloadError
func extractArchive(source io.ReadSeeker, target fileSystem) error {
	// Ensure we do buffered I/O
	br := bufio.NewReaderSize(source, 4096)
	head, _ := br.Peek(512)

	// If not a TAR archive we return a MalformedPayloadError
	// TODO: support other archive formats:
	//        golang.org/pkg/archive/zip
	//        github.com/nwaples/rardecode
	//        github.com/mkrautz/goar
	// TODO: support decompression from:
	//        golang.org/pkg/compress/gzip/
	//        golang.org/pkg/compress/bzip2/
	//        github.com/DataDog/zstd
	//        github.com/MediaMath/go-lzop
	//        lzma/xz, brotli, lz4, maybe too
	if !matchers.Tar(head) {
		kind, _ := filetype.Match(head)
		if kind.MIME.Value == "" {
			return runtime.NewMalformedPayloadError(
				"unable to detect cache preload data format, try TAR archives instead",
			)
		}
		return runtime.NewMalformedPayloadError(fmt.Sprintf(
			"caches cannot be pre-loaded with '%s', try TAR archives instead",
			kind.MIME.Value,
		))
	}

	// Wrap reader, so we can detect internal input errors, vs. tar-ball errors
	ebr := errorCapturingReader{Reader: br}
	tr := tar.NewReader(&ebr)

	// Extract tar-ball
	for {
		// Read an entry
		header, err := tr.Next()
		if ebr.Err != nil { // if there was an error reading from source it's internal
			return errors.Wrap(ebr.Err, "error reading from buffered archive")
		}
		if err == io.EOF { // if EOF, then we're done
			return nil
		}
		// If there was an error otherwise, it's a tar-ball error
		if err != nil {
			return runtime.NewMalformedPayloadError(fmt.Sprintf(
				"error reading TAR arcive: %s", err.Error(),
			))
		}

		info := header.FileInfo()
		if info.IsDir() {
			debug("extracting folder: '%s'", header.Name)

			err = target.WriteFolder(header.Name)
			if err != nil {
				return errors.Wrap(err, "Volume.WriteFolder() failed")
			}
		} else if info.Mode().IsRegular() {
			debug("extracting file: '%s'", header.Name)

			w := target.WriteFile(header.Name)
			// We capture errors from the reader, because we don't want these to become
			// internal errors.
			er := errorCapturingReader{Reader: tr}
			_, err = io.Copy(w, &er)
			if ebr.Err != nil {
				return errors.Wrap(ebr.Err, "error reading from buffered archive")
			}
			if er.Err != nil {
				w.Close()
				return runtime.NewMalformedPayloadError(fmt.Sprintf(
					"error reading TAR arcive: %s", er.Err.Error(),
				))
			}
			if err != nil {
				w.Close()
				return errors.Wrap(err, "failed to write file to io.WriteCloser from Volume.WriteFile()")
			}
			if err = w.Close(); err != nil {
				return errors.Wrap(err, "Volume.WriteFile().Close() failed")
			}
		} else {
			return runtime.NewMalformedPayloadError(fmt.Sprintf(
				"archive entry '%s' with fileMode: %s is not supported",
				info.Name(), info.Mode().String(),
			))
		}
	}
}
