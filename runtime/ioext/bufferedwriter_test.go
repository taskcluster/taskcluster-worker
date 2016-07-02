package ioext

import (
	"io"
	"testing"
)
import "bytes"

type writeNopCloser struct {
	io.Writer
}

func (writeNopCloser) Close() error {
	return nil
}

func TestBufferedWriter(t *testing.T) {
	raw := bytes.NewBuffer(nil)
	buf := NewBufferedWriteCloser(writeNopCloser{raw}, 10)
	_, err := buf.Write([]byte("Hi,"))
	if err != nil {
		t.Fatal("got error from Write", err)
	}
	_, err = buf.Write([]byte("Hello world"))
	if err != nil {
		t.Fatal("got error from Write", err)
	}
	buf.Close()
	if string(raw.Bytes()) != "Hi,Hello world" {
		t.Fatal("Got wrong text: ", string(raw.Bytes()))
	}
}
