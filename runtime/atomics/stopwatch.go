package atomics

import (
	"sync"
	"time"
)

// A StopWatch can be used to measure time. In contracts to using time.Duration
// this structure is thread-safe.
type StopWatch struct {
	m       sync.Mutex
	started time.Time
}

// Reset will stop the timer, reset it to zero and return the time elapsed
// before resetting.
func (s *StopWatch) Reset() time.Duration {
	s.m.Lock()
	defer s.m.Unlock()

	// If zero value then it's not started yet
	if s.started == (time.Time{}) {
		return 0
	}

	elapsed := time.Since(s.started)
	s.started = time.Time{}
	return elapsed
}

// Start the StopWatch, this will increase the elapsed time as time goes.
// This will not reset the StopWatch if it's already started.
func (s *StopWatch) Start() {
	s.m.Lock()
	defer s.m.Unlock()

	if s.started != (time.Time{}) {
		s.started = time.Now()
	}
}

// Elapsed returns the time elapsed since the StopWatch was started.
func (s *StopWatch) Elapsed() time.Duration {
	s.m.Lock()
	defer s.m.Unlock()

	// If zero value then it's not started yet
	if s.started == (time.Time{}) {
		return 0
	}

	return time.Since(s.started)
}
