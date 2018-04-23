package fetcher

import (
	"context"
	"errors"
	"io"
	"sync"
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
	ferr := reference.Fetch(context, s) // Notice that Fetch() may invoke s.Reset()
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
	wg      sync.WaitGroup
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

	// Wait for handler call to be done, so we don't call it concurrently
	// Even if calling concurrently wouldn't be a problem here, doing it is a
	// tiny optimization that only affects cases where we a broken connection
	// and Fetch() needs to reset the download process. And doing it would make
	// a lot harder to write a property StreamHandler as it would likely need to
	// deal with this concurrency.
	s.wg.Wait()

	ctx, cancel := context.WithCancel(context.Background())
	reader, writer := io.Pipe()
	s.reader = reader
	s.writer = writer
	s.ctx = ctx
	s.cancel = cancel
	s.err = nil

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer cancel()

		err := s.handler(ctx, reader)
		reader.CloseWithError(err)
		if ctx.Err() == nil {
			s.err = err
		}
	}()

	return nil
}

func (s *streamReseter) CloseWithError(err error) error {
	s.m.Lock()
	defer s.m.Unlock()

	// Ensure that the writer pipe is closed
	s.writer.CloseWithError(err) // ignore error, we don't care if it's closed twice

	// Wait for the handler call to be finished
	s.wg.Wait()

	return s.err
}
