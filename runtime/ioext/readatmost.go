package ioext

import (
	"errors"
	"io"
	"io/ioutil"
)

// A simple limitedReader similar to io.LimitReader, but this also let's us know
// if we reached EOF
type limitedReader struct {
	reader    io.Reader
	maxBytes  int64
	lastError error
}

func (l *limitedReader) Read(p []byte) (int, error) {
	// If we've reached maxBytes, we return EOF
	if l.maxBytes < 0 {
		return 0, io.EOF
	}
	// If buffer p is larger than maxBytes we slice it down...
	if int64(len(p)) > l.maxBytes {
		p = p[:l.maxBytes+1] // always offer to read one more, or we won't hit EOF
	}
	// Read into the buffer
	n, err := l.reader.Read(p)
	l.lastError = err // Notice that this is the only way lastError gets set
	l.maxBytes -= int64(n)

	return n, err
}

func (l *limitedReader) ReachedEOF() bool {
	// If the lastError isn't EOF or if we read more than maxBytes we're done
	return l.lastError == io.EOF && l.maxBytes >= 0
}

// ErrMaxSizeExceeded signals that EOF wasn't reached instead max size was
// was read and, hence, we stopped reading.
var ErrMaxSizeExceeded = errors.New("MaxSize was exceeded before EOF was reached")

// ReadAtMost will read at-most maxSize bytes from r and return an error if we
// didn't reach EOF. Returns ErrMaxSizeExceeded if r contains more than maxSize
// bytes. If maxSize is -1 ReadAtMost will read everything.
//
// This utility is useful when reading HTTP requests, in particular if reading
// from untrusted sources. If using io.ReadAll it's easy to run the server out
// of memory, a maxSize of 2MiB is usually sane and prevents such attacks.
func ReadAtMost(r io.Reader, maxSize int64) ([]byte, error) {
	if r == nil {
		return nil, nil
	}

	// If r.maxSize is zero or less read the entire body regardless of length
	if maxSize == -1 {
		return ioutil.ReadAll(r)
	}

	// Read at-most maxSize from body and check that we read it all
	lr := limitedReader{
		reader:   r,
		maxBytes: maxSize,
	}
	body, err := ioutil.ReadAll(&lr)
	if err != nil {
		return nil, err
	}
	if !lr.ReachedEOF() {
		return nil, ErrMaxSizeExceeded
	}
	return body, nil
}
