package runtime

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockStoppable struct {
	stopNow        chan struct{}
	stopGracefully chan struct{}
}

func (s *mockStoppable) StopNow() {
	close(s.stopNow)
}

func (s *mockStoppable) StopGracefully() {
	close(s.stopGracefully)
}

func isClosed(c <-chan struct{}) bool {
	select {
	case <-c:
		return true
	default:
		return false
	}
}

func TestStoppableOnceStopNowOnce(t *testing.T) {
	s := &mockStoppable{
		stopNow:        make(chan struct{}),
		stopGracefully: make(chan struct{}),
	}

	o := StoppableOnce{Stoppable: s}
	o.StopNow()
	o.StopNow()
	o.StopGracefully()
	o.StopGracefully()
	assert.True(t, isClosed(s.stopNow))
	assert.False(t, isClosed(s.stopGracefully))
}

func TestStoppableOnceStopNowOverwritesGracefully(t *testing.T) {
	s := &mockStoppable{
		stopNow:        make(chan struct{}),
		stopGracefully: make(chan struct{}),
	}

	o := StoppableOnce{Stoppable: s}
	o.StopGracefully()
	o.StopNow()
	o.StopNow()
	o.StopGracefully()
	o.StopNow()
	assert.True(t, isClosed(s.stopNow))
	assert.True(t, isClosed(s.stopGracefully))
}
