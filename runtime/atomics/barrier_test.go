package atomics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBarrier(t *testing.T) {
	b := Barrier{}
	assert.False(t, b.IsFallen())
	done := make(chan struct{})
	go func() {
		<-b.Barrier()
		close(done)
	}()
	done2 := false
	b.Forward(func() {
		done2 = true
	})
	time.Sleep(5 * time.Millisecond)
	select {
	case <-done:
		assert.FailNow(t, "should not be done")
	default:
	}
	assert.False(t, done2)
	assert.True(t, b.Fall())
	assert.True(t, b.IsFallen())
	<-done
	<-done
	assert.True(t, done2)
	assert.False(t, b.Fall())
	assert.False(t, b.Fall())
	assert.True(t, b.IsFallen())
	<-b.Barrier()
	<-b.Barrier()
	<-b.Barrier()
	<-b.Barrier()
}
