package ioext

import (
	"bytes"
	"errors"
	"io"
	"os"
)

// ErrFileTooBig is used the indicate that a file is too big.
var ErrFileTooBig = errors.New("File is larger than max size given")

// BoundedReadAll will read up to maxBytes from r, and returns
// ErrFileTooBig if the file is larger.
func BoundedReadAll(r io.Reader, maxBytes int) ([]byte, error) {
	buf := bytes.NewBuffer(nil)

	p := make([]byte, bytes.MinRead)
	var err error
	for err == nil {
		var n int
		n, err = r.Read(p)
		maxBytes -= n
		if maxBytes < 0 {
			return nil, ErrFileTooBig
		}
		buf.Write(p[:n])
	}
	if err != io.EOF {
		return nil, err
	}

	return buf.Bytes(), nil
}

// BoundedReadFile will read up to maxBytes from filename, and returns
// ErrFileTooBig if the file is larger, and *os.PathError if file doesn't exist.
func BoundedReadFile(filename string, maxBytes int) ([]byte, error) {
	// Open file for reading
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return BoundedReadAll(f, maxBytes)
}
