package ioext

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

type closeableWriter struct {
	io.Writer
	IsClosed bool
}

func (c *closeableWriter) Close() error {
	c.IsClosed = true
	return nil
}

func TestCopyAndClose(t *testing.T) {
	b := bytes.NewBuffer(nil)
	w := &closeableWriter{Writer: b}
	_, err := CopyAndClose(w, bytes.NewBufferString("Hello world"))
	require.NoError(t, err)
	require.True(t, w.IsClosed)
}
