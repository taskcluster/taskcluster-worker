package ioext

import "io"

// CopyAndClose will copy from r to w and close w, returning the number of bytes
// copied and the first error, if any, this always closes regardless of error.
func CopyAndClose(w io.WriteCloser, r io.Reader) (int64, error) {
	n, err1 := io.Copy(w, r)
	err2 := w.Close()
	if err1 != nil {
		return n, err1
	}
	return n, err2
}
