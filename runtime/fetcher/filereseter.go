package fetcher

import "io"

// File interface as implemented by *os.File
type File interface {
	Truncate(size int64) error
	io.Seeker
	io.Writer
}

// FileReseter implements WriteReseter for an *os.File instance
type FileReseter struct {
	File
}

// Reset will truncate the file and seek to the beginning.
func (f *FileReseter) Reset() error {
	err := f.Truncate(0)
	if err != nil {
		return err
	}
	_, err = f.Seek(0, 0)
	return err
}
