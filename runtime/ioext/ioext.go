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
func NopCloser(r io.Reader) ReadSeekCloser {
	//TODO: Require this cast be done outside of NopCloser!!!!
	return readSeekNopCloser{r.(io.ReadSeeker)}
}

type readSeekNopCloser struct {
	io.ReadSeeker
}

func (readSeekNopCloser) Close() error {
	return nil
}

type writeNopCloser struct {
	io.Writer
}

func (w *writeNopCloser) Close() error {
	return nil
}

// WriteNopCloser wraps an io.Writer and creates a io.WriteCloser where Close
// is a noop.
func WriteNopCloser(w io.Writer) io.WriteCloser {
	return &writeNopCloser{w}
}
