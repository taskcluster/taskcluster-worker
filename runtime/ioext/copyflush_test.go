package ioext

import (
	"bytes"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type flushBlockedWriter struct {
	m       sync.Mutex
	flushed bytes.Buffer
	buf     bytes.Buffer
}

func (w *flushBlockedWriter) Write(p []byte) (int, error) {
	w.m.Lock()
	defer w.m.Unlock()
	return w.buf.Write(p)
}

func (w *flushBlockedWriter) Flush() {
	w.m.Lock()
	defer w.m.Unlock()
	_, err := w.buf.WriteTo(&w.flushed)
	if err != nil {
		panic(fmt.Sprintf("Unexpected error: %s", err))
	}
	w.buf.Reset()
}

func (w *flushBlockedWriter) String() string {
	w.m.Lock()
	defer w.m.Unlock()
	return w.flushed.String()
}

func TestCopyAndFlush(t *testing.T) {
	b := &flushBlockedWriter{}
	in, out := io.Pipe()

	var err error
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		_, err = CopyAndFlush(b, in, 5*time.Millisecond)
		wg.Done()
	}()
	out.Write([]byte("Test"))
	for b.String() != "Test" {
		time.Sleep(1 * time.Millisecond)
	}
	out.Write([]byte("TestAgain"))
	out.Close()
	wg.Wait()
	require.NoError(t, err)

	// Check result
	require.Equal(t, "TestTestAgain", b.String())
}
