package fakequeue

// Listener provides and interface that allows waiting for task resolution.
//
// This is not intended to be a generic pulse listening interface, it's intended
// to the minimum subset required to write integration test. This way we can
// implement it using Pulse/AMQP for integration testing and using FakeQueue for
// testing without credentials.
type Listener interface {
	// WaitForTask returns a channel that gets a nil message when taskID is
	// resolved, or an error if listening fails.
	WaitForTask(taskID string) <-chan error
}

// NewFakeQueueListener returns a Listener implementation that waits for tasks
// to be resolved in the FakeQueue given.
func NewFakeQueueListener(q *FakeQueue) Listener {
	return &fakeQueueListener{queue: q}
}

type fakeQueueListener struct {
	queue *FakeQueue
}

func (l *fakeQueueListener) WaitForTask(taskID string) <-chan error {
	done := make(chan error)

	go func() {
		l.queue.initAndLock()
		defer l.queue.m.Unlock()

		for {
			t, ok := l.queue.tasks[taskID]
			if ok && (t.status.State == statusCompleted ||
				t.status.State == statusFailed ||
				t.status.State == statusException) {
				// close done, and return
				close(done)
				return
			}

			// Wait for a change
			l.queue.c.Wait()
		}
	}()

	return done
}
