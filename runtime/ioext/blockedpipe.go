package ioext

import (
	"io"
	"sync"
)

// PipeReader is a reading side of a BlockedPipe, similar to io.PipeReader.
type PipeReader struct {
	m         sync.Mutex
	c         sync.Cond
	unblocked int64
	r         *io.PipeReader
	rerr      error
}

// PipeWriter is a writing side of a BlockedPipe, similar to io.PipeWriter.
type PipeWriter struct {
	w *io.PipeWriter
}

// BlockedPipe is similar to io.Pipe() except the pipe is blocked until
// PipeReader.Unblock(n) is called to unblock n bytes. This is useful when
// implementing congestion control.
//
// Note: PipeReader.Unblock(-1) will permanently unblock the pipe,
// PipeReader.Close() will break the pipe, either one must be called unless the
// pipe is read EOF. Otherwise, the pipe will remain blocked and you may leak
// a go routine.
func BlockedPipe() (*PipeReader, *PipeWriter) {
	r, w := io.Pipe()
	reader := &PipeReader{
		r: r,
	}
	reader.c.L = &reader.m
	return reader, &PipeWriter{
		w: w,
	}
}

// Read mirrors io.PipeReader.Read() except it won't read more bytes than have
// been unblocked by calling r.Unblock().
func (r *PipeReader) Read(data []byte) (int, error) {
	// Wait for unblocked to be non-zero (-1 means infinite)
	r.m.Lock()
	defer r.m.Unlock()

	// Wait till unblocked isn't zero or we have a read error
	for r.unblocked == 0 && r.rerr == nil {
		r.c.Wait()
	}

	// Check if pipe was closed on the reading side
	if r.rerr != nil {
		// io.PipeReader always returns ErrClosedPipe regardless of what method err
		// the pipe was closed with, hence, PipeReader.CloseWithError only affects
		// the writing side of the pipe
		return 0, io.ErrClosedPipe
	}

	// If unblocked isn't -1 (infinite) and unblocked < len(data) then we have to
	// limit the length of data
	if r.unblocked < int64(len(data)) && r.unblocked != -1 {
		data = data[:r.unblocked]
	}

	// If unblocked isn't -1 (infinite) then we must subtract the bytes we've
	// decided to read
	if r.unblocked != -1 {
		r.unblocked -= int64(len(data))
		if r.unblocked < -1 {
			panic("blockedPipe.unblocked should never less than -1")
		}
	}

	// Read from underlying pipe (no need to lock when doing this)
	r.m.Unlock()
	n, err := r.r.Read(data)
	r.m.Lock()

	// When we've read we need to add back the unblocked bytes we reserved upfront
	// and subtract the number of bytes that we actually read...
	if r.unblocked != -1 {
		r.unblocked += int64(len(data)) - int64(n)
		// If unblocked is non-zero as a result we broadcasts a signal
		if r.unblocked != 0 {
			r.c.Broadcast()
		}
	}

	// Return what we've read
	return n, err
}

// Close mirrors io.PipeReader.Close()
func (r *PipeReader) Close() error {
	return r.CloseWithError(nil)
}

// CloseWithError mirrors io.PipeReader.CloseWithError()
func (r *PipeReader) CloseWithError(err error) error {
	if err == nil {
		err = io.ErrClosedPipe
	}

	// Set read error and wake sleeping routines
	r.m.Lock()
	r.rerr = err
	r.c.Broadcast()
	r.m.Unlock()

	return r.r.CloseWithError(err)
}

// Unblock allows n bytes to traverse through the pipe. Typically, this pipe is
// used when implementing congestion control and r.Unblock(n) is then called
// when the remote side have acknowleged n bytes. This way the network isn't
// congested with lots of outstanding bytes.
//
// As a special case n = -1 permanently unblocks the pipe.
//
// Note: That with this reader it is important to call r.Close() when cleaning
// up or r.Unblock(-1) as w.Close() won't propagate unless enough bytes are
// unblocked, and this could otherwise leave the pipe permanently blocked, which
// easily leaves you leaking go routines.
func (r *PipeReader) Unblock(n int64) {
	r.m.Lock()
	defer r.m.Unlock()

	if r.unblocked != -1 {
		blocked := r.unblocked == 0
		r.unblocked += n
		if blocked {
			r.c.Broadcast()
		}
	}
}

func (w *PipeWriter) Write(data []byte) (int, error) {
	return w.w.Write(data)
}

// Close mirrors io.PipeWriter.Close()
func (w *PipeWriter) Close() error {
	return w.w.Close()
}

// CloseWithError mirrors io.CloseWithError.CloseWithError()
func (w *PipeWriter) CloseWithError(err error) error {
	return w.w.CloseWithError(err)
}
