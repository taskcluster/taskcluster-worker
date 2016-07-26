package ioext

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"testing"
	"time"
)

func TestBlockedPipe(t *testing.T) {
	r, w := BlockedPipe()

	go func() {
		w.Write([]byte("12345678"))
		w.Close()
	}()

	done := make(chan struct{})
	allDone := make(chan struct{})
	go func() {
		p := make([]byte, 4)
		n, err := io.ReadFull(r, p)
		if n != 4 || string(p[:n]) != "1234" {
			fmt.Printf("read data: '%s', n = %d\n", string(p), n)
			panic("Wrong data read")
		}
		if err != nil {
			panic("Got an error already!!")
		}
		close(done)

		// Read to end
		p = make([]byte, 5)
		n, err = io.ReadFull(r, p)
		if n != 4 || string(p[:n]) != "5678" {
			fmt.Printf("read data: '%s', n = %d\n", string(p), n)
			panic("Wrong data read")
		}
		if err != io.ErrUnexpectedEOF {
			panic("Expected EOF")
		}

		close(allDone)
	}()

	select {
	case <-done:
		panic("Shouldn't have read anything yet")
	case <-time.After(50 * time.Millisecond):
	}

	// Let's unblock 4
	r.Unblock(4)

	// Okay now we should have read 1234
	<-done

	// Let's unblock everything
	r.Unblock(-1)

	<-allDone
}

func TestBlockedPipeClosedPipe(t *testing.T) {
	r, w := BlockedPipe()

	cleanedup := make(chan struct{})
	go func() {
		w.Write([]byte("12345678"))
		w.Close()
		close(cleanedup)
	}()

	done := make(chan struct{})
	allDone := make(chan struct{})
	go func() {
		p := make([]byte, 4)
		n, err := io.ReadFull(r, p)
		if n != 4 || string(p) != "1234" {
			panic("Wrong data read")
		}
		if err != nil {
			panic("Got an error already!!")
		}
		close(done)

		// Read to end
		p = make([]byte, 5)
		_, err = io.ReadFull(r, p)
		if err != io.ErrClosedPipe {
			panic("Expected ErrClosedPipe")
		}

		close(allDone)
	}()

	select {
	case <-done:
		panic("Shouldn't have read anything yet")
	case <-time.After(50 * time.Millisecond):
	}

	// Let's unblock 4
	r.Unblock(4)

	// Okay now we should have read 1234
	<-done

	select {
	case <-allDone:
		panic("Shouldn't be all done yet")
	case <-time.After(50 * time.Millisecond):
	}

	// Close the pipe
	fmt.Println("- Closing")
	if r.Close() != nil {
		panic("didn't think we should get an error here")
	}
	fmt.Println("- Closed")

	// Now we should be all done
	<-allDone
	fmt.Println("- allDone")

	// And we should have cleanedup, not leaking anything that would be bad
	<-cleanedup
	fmt.Println("- Cleanup")
}

func TestBlockedReachesEOF(t *testing.T) {
	r, w := BlockedPipe()
	r.Unblock(1024)

	go func() {
		w.Write([]byte("Hello"))
		w.Close()
	}()
	data, err := ioutil.ReadAll(r)
	if err != nil {
		panic(fmt.Sprintf("Unexpected error: %s", err))
	}
	if string(data) != "Hello" {
		panic("Expected to get 'Hello'")
	}
}

func TestBlockedHugeStream(t *testing.T) {
	r, w := BlockedPipe()
	r.Unblock(4096 + 4*1024*1024 + 7)

	input := make([]byte, 4*1024*1024+7)
	rand.Read(input)
	go func() {
		w.Write(input)
		w.Close()
	}()
	data, err := ioutil.ReadAll(r)
	if err != nil {
		panic(fmt.Sprintf("Unexpected error: %s", err))
	}
	if !bytes.Equal(input, data) {
		panic("Expected input to match data")
	}
}

func TestBlockedSlowStream(t *testing.T) {
	r, w := BlockedPipe()
	r.Unblock(4096)

	input := make([]byte, 4*1024*1024+7)
	rand.Read(input)
	go func() {
		w.Write(input)
		w.Close()
	}()
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				break
			case <-time.After(1 * time.Millisecond):
				r.Unblock(4096)
			}
		}
	}()
	data, err := ioutil.ReadAll(r)
	close(done)
	if err != nil {
		panic(fmt.Sprintf("Unexpected error: %s", err))
	}
	if !bytes.Equal(input, data) {
		panic("Expected input to match data")
	}
}
