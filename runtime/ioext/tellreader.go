package ioext

import (
	"io"
	"sync/atomic"
)

// TellReader is an io.Reader wrapper that can tell how much has been read.
//
// This is useful for monitoring download progress.
type TellReader struct {
	io.Reader
	n int64
}

func (r *TellReader) Read(p []byte) (n int, err error) {
	n, err = r.Reader.Read(p)
	atomic.AddInt64(&r.n, int64(n))
	return
}

// Tell the number bytes read so far
func (r *TellReader) Tell() int64 {
	return atomic.LoadInt64(&r.n)
}
