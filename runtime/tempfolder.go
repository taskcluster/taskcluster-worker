package runtime

import (
	"io"
	"os"
	"path/filepath"

	"github.com/taskcluster/slugid-go/slugid"
)

// TemporaryStorage can create temporary folders and files.
type TemporaryStorage interface {
	NewFolder() (TemporaryFolder, error)
	NewFile() (TemporaryFile, error)
}

// TemporaryFolder is a temporary folder that is backed by the filesystem.
// User are nicely asked to stay with the folder they've been issued.
//
// We don't really mock the file system interface as we need to integrate with
// other applications like docker, so we have to expose real file paths.
type TemporaryFolder interface {
	TemporaryStorage
	Path() string
	Remove() error
}

// TemporaryFile is a temporary file that will be removed when closed.
type TemporaryFile interface {
	io.ReadWriteSeeker
	io.Closer
	Path() string
}

type temporaryFolder struct {
	path string
}

type temporaryFile struct {
	*os.File
	path string
}

// NewTemporaryStorage TemporaryStorage rooted in the given path.
func NewTemporaryStorage(path string) (TemporaryStorage, error) {
	err := os.MkdirAll(path, 0644)
	if err != nil {
		return nil, err
	}
	return &temporaryFolder{path: path}, nil
}

func (s *temporaryFolder) Path() string {
	return s.path
}

func (s *temporaryFolder) NewFolder() (TemporaryFolder, error) {
	path := filepath.Join(s.path, slugid.V4())
	err := os.Mkdir(path, 0644)
	if err != nil {
		return nil, err
	}
	return &temporaryFolder{path: path}, nil
}

func (s temporaryFolder) NewFile() (TemporaryFile, error) {
	path := filepath.Join(s.path, slugid.V4())
	file, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	return &temporaryFile{file, path}, nil
}

func (s *temporaryFolder) Remove() error {
	return os.RemoveAll(s.path)
}

func (f *temporaryFile) Path() string {
	return f.path
}

func (f *temporaryFile) Close() error {
	f.File.Close()
	return os.Remove(f.path)
}
