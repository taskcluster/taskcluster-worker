package ioext

import (
	"io"
	"sync"
)

type watchPipe struct {
	pipe    io.ReadWriteCloser
	m       sync.Mutex
	onClose func(error)
}

// WatchPipe will wrap an io.ReadWriteCloser such that onClose is called when
// the first read or write error happens, or after close() is called.
//
// The onClose callback will be called with the error from read, write or close.
// To allow for locking for example to remove the pipe from a list when closed,
// the onClose callback is always called on in separate go-routine.
func WatchPipe(pipe io.ReadWriteCloser, onClose func(error)) io.ReadWriteCloser {
	return &watchPipe{
		pipe:    pipe,
		onClose: onClose,
	}
}

func (p *watchPipe) Read(b []byte) (int, error) {
	n, err := p.pipe.Read(b)

	// If there was an error we should call onClose
	if err != nil {
		// Take onClose (so we only call it once)
		p.m.Lock()
		onClose := p.onClose
		p.onClose = nil
		p.m.Unlock()

		// Call onClose, if we were able to take it
		if onClose != nil {
			go onClose(err)
		}
	}

	return n, err
}

func (p *watchPipe) Write(b []byte) (int, error) {
	n, err := p.pipe.Write(b)

	// If there was an error we should call onClose
	if err != nil {
		// Take onClose (so we only call it once)
		p.m.Lock()
		onClose := p.onClose
		p.onClose = nil
		p.m.Unlock()

		// Call onClose, if we were able to take it
		if onClose != nil {
			go onClose(err)
		}
	}

	return n, err
}

func (p *watchPipe) Close() error {
	// Close underlying pipe
	err := p.pipe.Close()

	// Take onClose (so we only call it once)
	p.m.Lock()
	onClose := p.onClose
	p.onClose = nil
	p.m.Unlock()

	// Call onClose, if we were able to take it
	if onClose != nil {
		go onClose(err)
	}

	return err
}
