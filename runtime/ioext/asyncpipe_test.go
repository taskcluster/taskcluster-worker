package ioext

import (
	"bytes"
	"crypto/rand"
	"io/ioutil"
	"testing"
)

func TestAsyncPipe(t *testing.T) {
	tell := make(chan int)
	told := 0
	done := make(chan struct{})
	go func() {
		for {
			n := <-tell
			if n == 0 {
				if 0 != <-tell {
					panic("I expect lots of zeros")
				}
				break
			}
			told += n
		}
		close(done)
	}()

	r, w := AsyncPipe(4*1024*1024, tell)

	var data []byte
	read := make(chan struct{})
	go func() {
		var err error
		data, err = ioutil.ReadAll(r)
		if err != nil {
			panic("Got an error")
		}
		close(read)
	}()

	if n, err := w.Write(make([]byte, 34554)); n != 34554 && err != nil {
		panic("Wrong result!")
	}
	if n, err := w.Write(make([]byte, 34)); n != 34 && err != nil {
		panic("Wrong result!")
	}
	if n, err := w.Write(make([]byte, 346)); n != 346 && err != nil {
		panic("Wrong result!")
	}
	if n, err := w.Write(make([]byte, 4096)); n != 4096 && err != nil {
		panic("Wrong result!")
	}
	w.Close()

	<-read
	if len(data) != 34554+34+346+4096 {
		panic("wrong length")
	}
	<-done
	if told != 34554+34+346+4096 {
		panic("told wrong")
	}
}

func TestAsyncPipeRandom(t *testing.T) {
	tell := make(chan int)
	told := 0
	done := make(chan struct{})
	go func() {
		for {
			n := <-tell
			if n == 0 {
				if 0 != <-tell {
					panic("I expect lots of zeros")
				}
				break
			}
			told += n
		}
		close(done)
	}()

	r, w := AsyncPipe(4*1024*1024, tell)

	var data []byte
	read := make(chan struct{})
	go func() {
		var err error
		data, err = ioutil.ReadAll(r)
		if err != nil {
			panic("Got an error")
		}
		close(read)
	}()

	input := make([]byte, 34554+34+346+4096)
	rand.Read(input)
	if n, err := w.Write(input[0:34554]); n != 34554 && err != nil {
		panic("Wrong result!")
	}
	if n, err := w.Write(input[34554 : 34554+34]); n != 34 && err != nil {
		panic("Wrong result!")
	}
	if n, err := w.Write(input[34554+34 : 34554+34+346]); n != 346 && err != nil {
		panic("Wrong result!")
	}
	if n, err := w.Write(input[34554+34+346 : 34554+34+346+4096]); n != 4096 && err != nil {
		panic("Wrong result!")
	}
	w.Close()

	<-read
	if len(data) != 34554+34+346+4096 {
		panic("wrong length")
	}
	if !bytes.Equal(data, input) {
		panic("Read the wrong data")
	}
	<-done
	if told != 34554+34+346+4096 {
		panic("told wrong")
	}
}

func TestAsyncPipeWithoutTell(t *testing.T) {
	r, w := AsyncPipe(4*1024*1024, nil)

	if n, err := w.Write(make([]byte, 34554)); n != 34554 && err != nil {
		panic("Wrong result!")
	}
	if n, err := w.Write(make([]byte, 34)); n != 34 && err != nil {
		panic("Wrong result!")
	}
	if n, err := w.Write(make([]byte, 346)); n != 346 && err != nil {
		panic("Wrong result!")
	}
	if n, err := w.Write(make([]byte, 4096)); n != 4096 && err != nil {
		panic("Wrong result!")
	}
	w.Close()

	data, err := ioutil.ReadAll(r)
	if err != nil {
		panic("Got an error")
	}

	if len(data) != 34554+34+346+4096 {
		panic("wrong length")
	}
}

func TestAsyncPipeCapicityHit(t *testing.T) {
	_, w := AsyncPipe(4096, nil)

	if n, err := w.Write(make([]byte, 4096)); n != 4096 && err != nil {
		panic("Wrong result!")
	}

	_, err := w.Write(make([]byte, 4096))
	if err != ErrPipeFull {
		panic("Expected ErrPipeFull")
	}
}
