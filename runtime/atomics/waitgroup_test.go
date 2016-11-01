package atomics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWaitGroup(t *testing.T) {
	wg := WaitGroup{}
	require.NoError(t, wg.Add(1))
	require.NoError(t, wg.Add(1))
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	time.Sleep(10 * time.Millisecond)
	require.NoError(t, wg.Add(1))
	wg.Done()
	wg.Drain()
	require.Error(t, wg.Add(1))
	require.Equal(t, ErrWaitGroupDraining, wg.Add(1))
	wg.Done()
	wg.Done()
	<-done
}

func TestWaitGroupDraining(t *testing.T) {
	wg := WaitGroup{}
	require.NoError(t, wg.Add(1))
	require.NoError(t, wg.Add(1))
	done := make(chan struct{})
	go func() {
		wg.WaitAndDrain()
		close(done)
	}()
	time.Sleep(10 * time.Millisecond)
	require.NoError(t, wg.Add(1))
	wg.Done()
	wg.Done()
	wg.Done()
	// Give time for other thread to get the lock and WaitAndDrain to do it's
	// thing
	time.Sleep(10 * time.Millisecond)
	require.Error(t, wg.Add(1))
	require.Equal(t, ErrWaitGroupDraining, wg.Add(1))
	<-done
}
