package ioext

import (
	"errors"
	"io"
	"sync"
)

// ErrPipeFull is returned from AsyncPipeWriter.Write if the pipes capacity has
// been reached.
var ErrPipeFull = errors.New("The internal pipe buffer have reached its capacity")

// AsyncPipeReader is a reading side of an AsyncPipe, similar to io.PipeReader.
type AsyncPipeReader struct {
	*AsyncPipeWriter
}

// AsyncPipeWriter is a writing side of a BlockedPipe, similar to io.PipeWriter.
type AsyncPipeWriter struct {
	c        sync.Cond
	m        sync.Mutex
	buffer   []byte
	closed   bool
	broken   bool
	tell     chan<- int
	capacity int
}

// AsyncPipe is similar to io.Pipe() except that writes isn't blocking, instead
// data will be added to an internal buffer that can grow up to specified
// capacity.
// Additionally, you may supply a channel tell that will be told whenever
// N bytes have been read, so that more bytes can be requested to be written.
//
// This pipe kind is useful when implementing congestion control.
func AsyncPipe(capacity int, tell chan<- int) (*AsyncPipeReader, *AsyncPipeWriter) {
	w := &AsyncPipeWriter{
		tell:     tell,
		capacity: capacity,
	}
	w.c.L = &w.m
	return &AsyncPipeReader{w}, w
}

func (r *AsyncPipeReader) Read(p []byte) (int, error) {
	r.m.Lock()
	defer r.m.Unlock()

	for len(r.buffer) == 0 && !r.closed {
		r.c.Wait()
	}

	// Copy to return value
	n := copy(p, r.buffer)

	// Move the rest of the buffer
	m := copy(r.buffer, r.buffer[n:])
	r.buffer = r.buffer[:m]

	// Tell how much has been read
	if n > 0 && r.tell != nil {
		r.tell <- n
	}

	// Set error to EOF, if closed and we're at the end of the buffer
	var err error
	if r.closed && len(r.buffer) == 0 {
		err = io.EOF
		if r.tell != nil {
			close(r.tell)
			r.tell = nil
		}
	}

	return n, err
}

// Close the pipe reader
func (r *AsyncPipeReader) Close() error {
	r.m.Lock()
	defer r.m.Unlock()
	r.closed = true
	r.broken = true
	r.c.Broadcast()
	return nil
}

func (w *AsyncPipeWriter) Write(p []byte) (int, error) {
	w.m.Lock()
	defer w.m.Unlock()

	// If pipe is closed we'll return an error
	if w.closed {
		return 0, io.ErrClosedPipe
	}

	// If we have more data than can fit the pipe buffer we return ErrPipeFull
	if len(w.buffer)+len(p) > w.capacity {
		return 0, ErrPipeFull
	}

	// Remember, if it was empty, so we know if we should signal
	empty := len(w.buffer) == 0

	// Append data
	w.buffer = append(w.buffer, p...)

	// Signal threads waiting, if we just added data
	if empty && len(p) > 0 {
		w.c.Broadcast()
	}

	return len(p), nil
}

// Close will close the stream
func (w *AsyncPipeWriter) Close() error {
	w.m.Lock()
	defer w.m.Unlock()

	if w.closed && w.broken {
		return io.ErrClosedPipe
	}

	w.closed = true
	w.c.Broadcast()
	return nil
}
