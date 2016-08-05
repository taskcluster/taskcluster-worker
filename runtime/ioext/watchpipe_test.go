package ioext

import (
	"bytes"
	"io"
	"io/ioutil"
	"net"
	"testing"
)

func TestWatchPipeReadTest(t *testing.T) {
	c1, c2 := net.Pipe()

	done := make(chan error, 1)

	c3 := WatchPipe(c1, func(err error) {
		done <- err
	})

	go io.Copy(ioutil.Discard, c3)

	c2.Close()
	if <-done != io.EOF {
		panic("Expected EOF!")
	}
}

func TestWatchPipeWriteTest(t *testing.T) {
	c1, c2 := net.Pipe()

	done := make(chan error, 1)

	c3 := WatchPipe(c1, func(err error) {
		done <- err
	})

	c2.Close()
	go io.Copy(c3, bytes.NewBufferString("Hello"))

	if <-done == nil {
		panic("Unexpected nil!")
	}
}

func TestWatchPipeCloseTest(t *testing.T) {
	c1, _ := net.Pipe()

	done := make(chan error, 1)

	c2 := WatchPipe(c1, func(err error) {
		done <- err
	})

	c2.Close()

	if <-done != nil {
		panic("Expected nil!")
	}
}
