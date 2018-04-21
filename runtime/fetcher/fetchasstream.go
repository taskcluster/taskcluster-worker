package fetcher

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
)

// ErrStreamReset returned from io.Reader when the fetch process is reset
var ErrStreamReset = errors.New("stream was reset by fetcher")

// A StreamHandler is a function that handles a stream, the stream maybe a
// aborted in which case io.Reader will return ErrStreamReset and the Context
// will be canceled.
type StreamHandler func(context.Context, io.Reader) error

// FetchAsStream gets a reference as a stream.
//
// Notice that target StreamHandler may be invoked multiple times, if the
// connection breaks while fetching it might be retried. In which case the
// Context passed to the target StreamHandler will be canceled.
func FetchAsStream(context Context, reference Reference, target StreamHandler) error {
	s := &streamReseter{handler: target}
	s.Reset() // initialize

	// fetch and close after fetching
	ferr := reference.Fetch(context, s)
	cerr := s.CloseWithError(ferr)

	if ferr != nil {
		return ferr
	}
	return cerr
}

type streamReseter struct {
	m       sync.Mutex
	handler StreamHandler
	reader  *io.PipeReader
	writer  *io.PipeWriter
	ctx     context.Context
	cancel  func()
	done    atomics.Once // covers err
	err     error
}

func (s *streamReseter) Write(p []byte) (n int, err error) {
	s.m.Lock()
	defer s.m.Unlock()

	return s.writer.Write(p)
}

func (s *streamReseter) Reset() error {
	s.m.Lock()
	defer s.m.Unlock()

	// Discard current state if any
	if s.cancel != nil {
		s.cancel()
		s.writer.CloseWithError(ErrStreamReset)
	}

	ctx, cancel := context.WithCancel(context.Background())
	reader, writer := io.Pipe()
	s.reader = reader
	s.writer = writer
	s.ctx = ctx
	s.cancel = cancel

	go func() {
		defer cancel()

		err := s.handler(ctx, reader)
		reader.CloseWithError(err)

		s.m.Lock()
		defer s.m.Unlock()
		if ctx.Err() == nil {
			s.err = err
			s.done.Do(nil)
		}
	}()

	return nil
}

func (s *streamReseter) CloseWithError(err error) error {
	s.m.Lock()
	defer s.m.Unlock()

	// Ensure that the writer pipe is closed
	s.writer.CloseWithError(err) // ignore error, we don't care if it's closed twice

	// Wait for the most recent handler call to be finished
	s.m.Unlock()
	s.done.Wait()
	s.m.Lock()

	return s.err
}
