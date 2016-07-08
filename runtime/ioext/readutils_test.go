package ioext

import (
	"bytes"
	"testing"
)

func TestBoundedReadAll(t *testing.T) {
	expected := []byte("Hello World")
	r := bytes.NewBuffer(expected)

	// Try to read something
	b, err := BoundedReadAll(r, 100)
	if err != nil {
		t.Fatal("Failed to read 11 bytes, error: ", err)
	}
	if bytes.Compare(b, expected) != 0 {
		t.Fatal("Read the wrong thing: ", b, " expected: ", expected)
	}

	// Try to read with tight bound
	r = bytes.NewBuffer(expected)
	b, err = BoundedReadAll(r, 11)
	if err != nil {
		t.Fatal("Failed to read 11 bytes, error: ", err)
	}
	if bytes.Compare(b, expected) != 0 {
		t.Fatal("Read the wrong thing: ", b, " expected: ", expected)
	}

	// Try to read with low bound
	r = bytes.NewBuffer(expected)
	b, err = BoundedReadAll(r, 5)
	if err != ErrFileTooBig {
		t.Fatal("Expected an ErrFileTooBig error with limit of 5")
	}
	if b != nil {
		t.Fatal("Expected nil")
	}

	// Try to read with tight low bound
	r = bytes.NewBuffer(expected)
	b, err = BoundedReadAll(r, 10)
	if err != ErrFileTooBig {
		t.Fatal("Expected an ErrFileTooBig error with limit of 10")
	}
	if b != nil {
		t.Fatal("Expected nil")
	}
}
