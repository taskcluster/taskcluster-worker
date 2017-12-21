package caching

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
	"github.com/taskcluster/taskcluster-worker/runtime/gc"
)

type opts struct {
	Sleep int  `json:"sleep"`
	Value int  `json:"value"`
	Error bool `json:"error"`
}

type res struct {
	sync.Mutex
	Value    int
	Disposed bool
}

func (r *res) MemorySize() (uint64, error) {
	return 0, ErrDisposableSizeNotSupported
}

func (r *res) DiskSize() (uint64, error) {
	return 0, ErrDisposableSizeNotSupported
}

func (r *res) Dispose() error {
	r.Lock()
	defer r.Unlock()

	if r.Disposed {
		panic("resource disposed twice!!!")
	}
	r.Disposed = true
	return nil
}

func constructor(ctx Context, options interface{}) (Resource, error) {
	opt, ok := options.(opts)
	if !ok {
		return nil, errors.New("invalid options")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Duration(opt.Sleep) * time.Millisecond):
	}

	if opt.Error {
		return nil, errors.New("some error")
	}

	return &res{Value: opt.Value}, nil
}

type tracker struct {
	sync.Mutex
	resources []gc.Disposable
}

func (t *tracker) Register(resource gc.Disposable) {
	t.Lock()
	defer t.Unlock()

	t.resources = append(t.resources, resource)
}

func (t *tracker) Unregister(resource gc.Disposable) bool {
	t.Lock()
	defer t.Unlock()

	found := false
	res := t.resources[:0]
	for _, r := range t.resources {
		if r != resource {
			res = append(res, r)
		} else {
			found = true
		}
	}
	t.resources = res

	return found
}

type mockctx struct {
	context.Context
}

func (c *mockctx) Progress(description string, percent float64) {
	debug("progress: %s -- %f %%", description, percent)
}

func TestSharedCache(t *testing.T) {
	var tr tracker
	c := New(constructor, true, &tr)
	require.Equal(t, 0, len(tr.resources), "expected zero resources")

	t.Run("single resource", func(t *testing.T) {
		debug("creating resource")
		handle, err := c.Require(&mockctx{context.Background()}, opts{
			Sleep: 0,
			Value: 0,
		})
		require.NoError(t, err)

		debug("reading resource")
		r, ok := handle.Resource().(*res)
		require.True(t, ok)
		r.Lock()
		require.Equal(t, 0, r.Value)
		r.Value++
		r.Unlock()

		require.Equal(t, 1, len(tr.resources), "expected one resource")
		handle.Release()
		handle.Release() // sanity check, why not

		debug("loading resource")
		handle, err = c.Require(&mockctx{context.Background()}, opts{
			Sleep: 0,
			Value: 0,
		})
		require.NoError(t, err)

		debug("reading resource")
		r, ok = handle.Resource().(*res)
		require.True(t, ok)
		r.Lock()
		require.Equal(t, 1, r.Value)
		r.Unlock()

		require.Equal(t, 1, len(tr.resources), "expected one resource")
		handle.Release()
		handle.Release() // sanity check, why not

		debug("purging")
		c.Purge(func(r Resource) bool {
			debug("deciding to purge: %#v", r)
			return true
		})
		time.Sleep(10 * time.Millisecond)

		debug("checking things were disposed")
		r.Lock()
		require.True(t, r.Disposed)
		r.Unlock()
		tr.Lock()
		require.Equal(t, 0, len(tr.resources), "expected zero resources")
		tr.Unlock()
	})

	t.Run("two resources", func(t *testing.T) {
		var hA *Handle
		var err2 error

		debug("creating resourceA")
		var done atomics.Once
		go done.Do(func() {
			hA, err2 = c.Require(&mockctx{context.Background()}, opts{
				Sleep: 100,
				Value: 0,
			})
			debug("created resourceA")
		})
		debug("creating resourceB")
		hB, err := c.Require(&mockctx{context.Background()}, opts{
			Sleep: 100,
			Value: 0,
		})
		require.NoError(t, err)
		debug("waiting for resourceA")
		done.Wait()
		require.NoError(t, err2)

		debug("comparing resources")
		require.Equal(t, hA.Resource(), hB.Resource())
		require.True(t, hA.Resource() == hB.Resource())

		debug("releasing resources")
		hA.Release()
		hB.Release()
	})

	t.Run("purge while in use", func(t *testing.T) {
		handle, err := c.Require(&mockctx{context.Background()}, opts{
			Sleep: 1,
			Value: 10,
		})
		require.NoError(t, err)

		debug("reading resource")
		r, ok := handle.Resource().(*res)
		require.True(t, ok)
		r.Lock()
		require.Equal(t, 10, r.Value)
		r.Unlock()

		// Increment to try and make a race condition
		var incremented atomics.Once
		go incremented.Do(func() {
			r.Value += 2
			time.Sleep(5 * time.Millisecond)
			r.Value += 2
			time.Sleep(5 * time.Millisecond)
			r.Value += 2
			r.Lock()
			require.False(t, r.Disposed)
			r.Unlock()
		})

		debug("purging")
		c.Purge(func(r Resource) bool {
			return true
		})
		time.Sleep(10 * time.Millisecond)

		debug("Wait for incremented to be done")
		incremented.Wait()

		debug("checking things were NOT disposed")
		r.Lock()
		require.False(t, r.Disposed)
		r.Unlock()

		debug("release resource")
		handle.Release()
		handle.Release() // sanity check, why not
		time.Sleep(10 * time.Millisecond)

		debug("checking things were disposed")
		r.Lock()
		require.True(t, r.Disposed)
		r.Unlock()
	})

	t.Run("two resources, create-error", func(t *testing.T) {
		var err2 error

		debug("creating resourceA")
		var done atomics.Once
		go done.Do(func() {
			_, err2 = c.Require(&mockctx{context.Background()}, opts{
				Sleep: 100,
				Value: 0,
				Error: true,
			})
			debug("created resourceA")
		})
		debug("creating resourceB")
		_, err := c.Require(&mockctx{context.Background()}, opts{
			Sleep: 100,
			Value: 0,
			Error: true,
		})
		require.Error(t, err)
		debug("waiting for resourceA")
		done.Wait()
		require.Error(t, err2)

		debug("comparing errors")
		require.Equal(t, err, err2)
		require.True(t, err == err2)

		tr.Lock()
		require.Equal(t, 0, len(tr.resources), "expected zero resources")
		tr.Unlock()
	})

	t.Run("one resource, create-canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		time.AfterFunc(10*time.Millisecond, cancel)

		debug("creating resourceA")
		_, err := c.Require(&mockctx{ctx}, opts{
			Sleep: 50,
			Value: 0,
		})
		require.Error(t, err)

		debug("comparing errors")
		require.Equal(t, context.Canceled, err)

		tr.Lock()
		require.Equal(t, 0, len(tr.resources), "expected zero resources")
		tr.Unlock()
	})
}

func TestExclusiveCache(t *testing.T) {
	var tr tracker
	c := New(constructor, false, &tr)
	require.Equal(t, 0, len(tr.resources), "expected zero resources")

	t.Run("single resource", func(t *testing.T) {
		handle, err := c.Require(&mockctx{context.Background()}, opts{
			Sleep: 0,
			Value: 0,
		})
		require.NoError(t, err)

		debug("reading resource")
		r, ok := handle.Resource().(*res)
		require.True(t, ok)
		r.Lock()
		require.Equal(t, 0, r.Value)
		r.Unlock()

		require.Equal(t, 1, len(tr.resources), "expected one resource")
		handle.Release()
		handle.Release() // sanity check, why not

		debug("purging")
		c.Purge(func(r Resource) bool {
			debug("deciding to purge: %#v", r)
			return true
		})
		time.Sleep(10 * time.Millisecond)

		debug("checking things were disposed")
		r.Lock()
		require.True(t, r.Disposed)
		r.Unlock()
		tr.Lock()
		require.Equal(t, 0, len(tr.resources), "expected zero resources")
		tr.Unlock()
	})

	t.Run("two resources", func(t *testing.T) {
		var hA *Handle
		var err2 error

		debug("creating resourceA")
		var done atomics.Once
		go done.Do(func() {
			hA, err2 = c.Require(&mockctx{context.Background()}, opts{
				Sleep: 100,
				Value: 0,
			})
			debug("created resourceA")
		})
		debug("creating resourceB")
		hB, err := c.Require(&mockctx{context.Background()}, opts{
			Sleep: 100,
			Value: 0,
		})
		require.NoError(t, err)
		debug("waiting for resourceA")
		done.Wait()
		require.NoError(t, err2)

		debug("comparing resources")
		require.False(t, hA.Resource() == hB.Resource())

		debug("releasing resources")
		hA.Release()
		hB.Release()
	})

	t.Run("purge while in use", func(t *testing.T) {
		handle, err := c.Require(&mockctx{context.Background()}, opts{
			Sleep: 1,
			Value: 10,
		})
		require.NoError(t, err)

		debug("reading resource")
		r, ok := handle.Resource().(*res)
		require.True(t, ok)
		r.Lock()
		require.Equal(t, 10, r.Value)
		r.Unlock()

		// Increment to try and make a race condition
		var incremented atomics.Once
		go incremented.Do(func() {
			r.Value += 2
			time.Sleep(5 * time.Millisecond)
			r.Value += 2
			time.Sleep(5 * time.Millisecond)
			r.Value += 2
			r.Lock()
			require.False(t, r.Disposed)
			r.Unlock()
		})

		debug("purging")
		c.Purge(func(r Resource) bool {
			return true
		})
		time.Sleep(10 * time.Millisecond)

		debug("Wait for incremented to be done")
		incremented.Wait()

		debug("checking things were NOT disposed")
		r.Lock()
		require.False(t, r.Disposed)
		r.Unlock()

		debug("release resource")
		handle.Release()
		handle.Release() // sanity check, why not
		time.Sleep(10 * time.Millisecond)

		debug("checking things were disposed")
		r.Lock()
		require.True(t, r.Disposed)
		r.Unlock()
	})

	t.Run("two resources, create-error", func(t *testing.T) {
		var err2 error

		debug("creating resourceA")
		var done atomics.Once
		go done.Do(func() {
			_, err2 = c.Require(&mockctx{context.Background()}, opts{
				Sleep: 100,
				Value: 0,
				Error: true,
			})
			debug("created resourceA")
		})
		debug("creating resourceB")
		_, err := c.Require(&mockctx{context.Background()}, opts{
			Sleep: 100,
			Value: 0,
			Error: true,
		})
		require.Error(t, err)
		debug("waiting for resourceA")
		done.Wait()
		require.Error(t, err2)

		debug("comparing errors")
		require.False(t, err == err2)

		tr.Lock()
		require.Equal(t, 0, len(tr.resources), "expected zero resources")
		tr.Unlock()
	})

	t.Run("one resource, create-canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		time.AfterFunc(10*time.Millisecond, cancel)

		debug("creating resourceA")
		_, err := c.Require(&mockctx{ctx}, opts{
			Sleep: 50,
			Value: 0,
		})
		require.Error(t, err)

		debug("comparing errors")
		require.Equal(t, context.Canceled, err)

		tr.Lock()
		require.Equal(t, 0, len(tr.resources), "expected zero resources")
		tr.Unlock()
	})
}
