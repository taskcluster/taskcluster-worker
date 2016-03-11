package ioext

import (
	"io"
)

// ReadSeekCloser implements io.Reader, io.Seeker, and io.Closer. It is trivially implemented by os.File.
type ReadSeekCloser interface {
	io.Reader
	io.Seeker
	io.Closer
}

func NopCloser(r io.Reader) ReadSeekNopCloser {
	return ReadSeekNopCloser{r.(io.ReadSeeker)}
}

type ReadSeekNopCloser struct {
	io.ReadSeeker
}

func (ReadSeekNopCloser) Close() error {
	return nil
}
