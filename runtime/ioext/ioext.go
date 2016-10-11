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

// NopCloser wraps a io.ReadSeeker as ReadSeekCloser with Close being a noop.
// This is useful for compliance with interface, if you don't care about closing.
func NopCloser(r io.ReadSeeker) ReadSeekCloser {
	return readSeekNopCloser{r}
}

type readSeekNopCloser struct {
	io.ReadSeeker
}

func (readSeekNopCloser) Close() error {
	return nil
}
