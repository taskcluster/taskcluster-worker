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

// NopCloser is useful in testing where something that implements ReadSeekCloser is needed
// If something that implements io.ReadSeeker is passed in, it will give it a noop close function.
func NopCloser(r io.Reader) ReadSeekNopCloser {
	return ReadSeekNopCloser{r.(io.ReadSeeker)}
}

// ReadSeekNopCloser is an implementation of ReadSeekCloser that wraps io.ReadSeekers
type ReadSeekNopCloser struct {
	io.ReadSeeker
}

// Close is a noop Close function that does nothing. Useful in testing.
func (ReadSeekNopCloser) Close() error {
	return nil
}
