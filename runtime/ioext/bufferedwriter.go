package ioext

import (
	"bufio"
	"io"
)

// A BufferedWriteCloser is the same as bufio.Writer except you can call Close()
// on the BufferedWriteCloser which will then make sure to Flush() the
// underlying bufio.Writer.
type BufferedWriteCloser struct {
	buf *bufio.Writer
	raw io.WriteCloser
}

// NewBufferedWriteCloser creates a BufferedWriteCloser with buffer of given
// size, similar to bufio.NewWriterSize.
func NewBufferedWriteCloser(w io.WriteCloser, size int) *BufferedWriteCloser {
	return &BufferedWriteCloser{
		buf: bufio.NewWriterSize(w, size),
		raw: w,
	}
}

func (b *BufferedWriteCloser) Write(p []byte) (nn int, err error) {
	return b.buf.Write(p)
}

// Close will flush buffered bytes and close the underlying io.WriteCloser
func (b *BufferedWriteCloser) Close() error {
	err1 := b.buf.Flush()
	err2 := b.raw.Close()
	if err1 != nil {
		return err1
	}
	return err2
}
