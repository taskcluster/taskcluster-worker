package ioext

import (
	"bytes"
	"testing"
)

func TestReadAtmost(t *testing.T) {
	r := bytes.NewBufferString("hello world")
	b, err := ReadAtmost(r, 20)
	if err != nil {
		t.Error("maxSize > N, failed")
	}
	if string(b) != "hello world" {
		t.Error("Read the wrong thing")
	}

	// MaxSize == N
	r = bytes.NewBufferString("hello world")
	b, err = ReadAtmost(r, 11)
	if err != nil {
		t.Error("maxSize == N, failed")
	}
	if string(b) != "hello world" {
		t.Error("Read the wrong thing")
	}

	// MaxSize < N
	r = bytes.NewBufferString("hello world")
	b, err = ReadAtmost(r, 10)
	if err != ErrMaxSizeExceeded {
		t.Error("maxSize < N, didn't fail")
	}
	if b != nil {
		t.Error("Expected b = nil")
	}

	// MaxSize < N
	r = bytes.NewBufferString("hello world")
	b, err = ReadAtmost(r, 4)
	if err != ErrMaxSizeExceeded {
		t.Error("maxSize < N, didn't fail (2)")
	}
	if b != nil {
		t.Error("Expected b = nil")
	}

	// MaxSize == -1
	r = bytes.NewBufferString("hello world")
	b, err = ReadAtmost(r, -1)
	if err != nil {
		t.Error("maxSize == -1, failed")
	}
	if string(b) != "hello world" {
		t.Error("Read the wrong thing")
	}
}
