package fetcher

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
)

type mockContext struct {
	context.Context
	queue client.Queue
	t     *testing.T
}

func (c *mockContext) Queue() client.Queue {
	return c.queue
}

func (c *mockContext) Progress(description string, percent float64) {
	c.t.Logf("Progress: %s - %d %%", description, percent)
}

type mockWriteSeekReseter struct {
	offset int64
	buffer []byte
}

func (w *mockWriteSeekReseter) Write(p []byte) (int, error) {
	offset := w.offset + int64(len(p))
	if int64(len(w.buffer)) < offset {
		w.buffer = append(w.buffer, make([]byte, offset-int64(len(w.buffer)))...)
	}
	copy(w.buffer[w.offset:], p)
	w.offset = offset
	return len(p), nil
}

func (w *mockWriteSeekReseter) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		offset += w.offset
	case io.SeekEnd:
		offset += int64(len(w.buffer))
	default:
		panic("whence value not supported")
	}

	// Check boundary
	if offset < 0 {
		return w.offset, fmt.Errorf("Can't seek to negative offset: %d", offset)
	}
	if offset > int64(len(w.buffer)) {
		panic("Seeking past end of file is implementation defined behavior, don't!")
	}
	w.offset = offset

	return w.offset, nil
}

func (w *mockWriteSeekReseter) Reset() error {
	w.offset = 0
	w.buffer = nil
	return nil
}

func (w *mockWriteSeekReseter) String() string {
	return string(w.buffer)
}

func TestMockWriteSeekReseter(t *testing.T) {
	w := &mockWriteSeekReseter{}
	_, err := io.Copy(w, bytes.NewBufferString("test"))
	require.NoError(t, err)
	require.Equal(t, w.String(), "test")

	err = w.Reset()
	require.NoError(t, err)
	_, err = io.Copy(w, bytes.NewBufferString("test again"))
	require.NoError(t, err)
	require.Equal(t, w.String(), "test again")

	_, err = io.Copy(w, bytes.NewBufferString(" test again"))
	require.NoError(t, err)
	require.Equal(t, w.String(), "test again test again")

	// Seek start
	_, err = w.Seek(0, io.SeekStart)
	require.NoError(t, err)
	_, err = io.Copy(w, bytes.NewBufferString("TEST again"))
	require.NoError(t, err)
	require.Equal(t, w.String(), "TEST again test again")

	// Seek end with offset
	_, err = w.Seek(-5, io.SeekEnd)
	require.NoError(t, err)
	_, err = io.Copy(w, bytes.NewBufferString("AGAIN"))
	require.NoError(t, err)
	require.Equal(t, w.String(), "TEST again test AGAIN")

	// Seek end
	_, err = w.Seek(0, io.SeekEnd)
	require.NoError(t, err)
	_, err = io.Copy(w, bytes.NewBufferString("!"))
	require.NoError(t, err)
	require.Equal(t, w.String(), "TEST again test AGAIN!")

	// Seek start with offset + seek current
	_, err = w.Seek(0, io.SeekStart)
	require.NoError(t, err)
	_, err = w.Seek(5, io.SeekCurrent)
	require.NoError(t, err)
	_, err = io.Copy(w, bytes.NewBufferString("-----"))
	require.NoError(t, err)
	require.Equal(t, w.String(), "TEST ----- test AGAIN!")
}
