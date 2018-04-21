package fetcher

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
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

type fakeReference struct {
	ScopesValue  [][]string
	HashKeyValue string
	Data         []byte
	Reset        bool
	Err          error
}

func (r *fakeReference) HashKey() string {
	return r.HashKeyValue
}

func (r *fakeReference) Scopes() [][]string {
	return r.ScopesValue
}

func (r *fakeReference) Fetch(context Context, target WriteReseter) error {
	i := 0
	// Write a few small blocks
	if i+1 < len(r.Data) {
		n, err := target.Write(r.Data[i : i+1])
		if err != nil {
			return err
		}
		if n != 1 {
			panic(errors.New("Expected n = 1 when no error was returned"))
		}
		i += n
	}
	if i+7 < len(r.Data) {
		n, err := target.Write(r.Data[i : i+7])
		if err != nil {
			return err
		}
		if n != 7 {
			panic(errors.New("Expected n = 7 when no error was returned"))
		}
		i += n
	}
	if i+3 < len(r.Data) {
		n, err := target.Write(r.Data[i : i+3])
		if err != nil {
			return err
		}
		if n != 3 {
			panic(errors.New("Expected n = 3 when no error was returned"))
		}
		i += n
	}
	if i < len(r.Data) {
		n, err := target.Write(r.Data[i:])
		if err != nil {
			return err
		}
		if n != len(r.Data)-i {
			panic(errors.New("Expected n = len(r.Data)-i when no error was returned"))
		}
	}
	if r.Reset {
		r.Reset = false
		if err := target.Reset(); err != nil {
			return nil
		}
		return r.Fetch(context, target)
	}
	return r.Err
}

func TestFakeReference(t *testing.T) {
	ctx := &fakeContext{Context: context.Background()}
	// Create a random blob
	blob := make([]byte, 16*1024+27)
	_, rerr := rand.Read(blob)
	require.NoError(t, rerr, "failed to created random data for testing")

	t.Run("Empty String without Reset", func(t *testing.T) {
		r := &fakeReference{
			Data:  []byte(""),
			Reset: false,
			Err:   nil,
		}
		w := &fakeWriteReseter{}
		err := r.Fetch(ctx, w)
		require.NoError(t, err)
		require.Equal(t, "", w.String())
	})

	t.Run("hello-world without Reset", func(t *testing.T) {
		r := &fakeReference{
			Data:  []byte("hello-world"),
			Reset: false,
			Err:   nil,
		}
		w := &fakeWriteReseter{}
		err := r.Fetch(ctx, w)
		require.NoError(t, err)
		require.Equal(t, "hello-world", w.String())
	})

	t.Run("blob without Reset", func(t *testing.T) {
		r := &fakeReference{
			Data:  blob,
			Reset: false,
			Err:   nil,
		}
		w := &fakeWriteReseter{}
		err := r.Fetch(ctx, w)
		require.NoError(t, err)
		require.True(t, bytes.Compare(blob, w.buffer) == 0, "expected blob == w.buffer")
	})

	t.Run("Empty String with Reset", func(t *testing.T) {
		r := &fakeReference{
			Data:  []byte(""),
			Reset: true,
			Err:   nil,
		}
		w := &fakeWriteReseter{}
		err := r.Fetch(ctx, w)
		require.NoError(t, err)
		require.Equal(t, "", w.String())
	})

	t.Run("hello-world with Reset", func(t *testing.T) {
		r := &fakeReference{
			Data:  []byte("hello-world"),
			Reset: true,
			Err:   nil,
		}
		w := &fakeWriteReseter{}
		err := r.Fetch(ctx, w)
		require.NoError(t, err)
		require.Equal(t, "hello-world", w.String())
	})

	t.Run("blob with Reset", func(t *testing.T) {
		r := &fakeReference{
			Data:  blob,
			Reset: true,
			Err:   nil,
		}
		w := &fakeWriteReseter{}
		err := r.Fetch(ctx, w)
		require.NoError(t, err)
		require.True(t, bytes.Compare(blob, w.buffer) == 0, "expected blob == w.buffer")
	})
}
