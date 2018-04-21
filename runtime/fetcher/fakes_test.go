package fetcher

import (
	"bytes"
	"context"
	"io"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
)

type fakeContext struct {
	context.Context
	queue           client.Queue
	m               sync.Mutex
	progressReports []float64
}

func (c *fakeContext) Queue() client.Queue {
	return c.queue
}

func (c *fakeContext) Progress(description string, percent float64) {
	c.m.Lock()
	defer c.m.Unlock()
	debug("Progress: %s - %.02f %%", description, percent*100)
	c.progressReports = append(c.progressReports, percent)
}

func (c *fakeContext) ProgressReports() []float64 {
	c.m.Lock()
	defer c.m.Unlock()
	return c.progressReports
}

type fakeWriteReseter struct {
	offset int64
	buffer []byte
}

func (w *fakeWriteReseter) Write(p []byte) (int, error) {
	offset := w.offset + int64(len(p))
	if int64(len(w.buffer)) < offset {
		w.buffer = append(w.buffer, make([]byte, offset-int64(len(w.buffer)))...)
	}
	copy(w.buffer[w.offset:], p)
	w.offset = offset
	return len(p), nil
}

func (w *fakeWriteReseter) Reset() error {
	w.offset = 0
	w.buffer = nil
	return nil
}

func (w *fakeWriteReseter) String() string {
	return string(w.buffer)
}

func TestFakeWriteSeekReseter(t *testing.T) {
	w := &fakeWriteReseter{}
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
}
