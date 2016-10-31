package fetcher

import "os"

// FileReseter implements WriteSeekReseter for an *os.File instance
type FileReseter struct {
	*os.File
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
