package lifecyclepolicy

import (
	"sync"

	"github.com/taskcluster/taskcluster-worker/runtime"
)

// StoppableOnce is a wrapper that ensures we only call StopGracefully and
// StopNow once and never call StopGracefully after StopNow.
//
// There is never any harm in wrapping with this, it merely limits excessive
// calls to StopNow() and StopGracefully(). Please note that Stoppable.StopNow()
// may still be invoked after Stoppable.StopGracefully(), it can even be invoked
// concurrently.
type StoppableOnce struct {
	Stoppable          runtime.Stoppable
	m                  sync.Mutex
	stoppingNow        chan struct{}
	stoppingGracefully chan struct{}
}

// StopGracefully calls StopGracefully() on the s.Stoppable, if neither
// StopGracefully() or StopNow() have been called.
func (s *StoppableOnce) StopGracefully() {
	s.m.Lock()
	if s.stoppingGracefully == nil && s.stoppingNow == nil {
		s.stoppingGracefully = make(chan struct{})
		go func() {
			s.Stoppable.StopGracefully()
			close(s.stoppingGracefully)
		}()
	}
	var stopped chan struct{}
	if s.stoppingNow != nil {
		stopped = s.stoppingNow
	} else {
		stopped = s.stoppingGracefully
	}
	s.m.Unlock()

	<-stopped
}

// StopNow calls StopNow() on s.Stoppable, if StopNow() haven't been called yet.
func (s *StoppableOnce) StopNow() {
	s.m.Lock()
	if s.stoppingNow == nil {
		s.stoppingNow = make(chan struct{})
		go func() {
			s.Stoppable.StopNow()
			close(s.stoppingNow)
		}()
	}
	s.m.Unlock()

	<-s.stoppingNow
}
