package webhookserver

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func assert(condition bool, a ...interface{}) {
	if !condition {
		panic(fmt.Sprintln(a...))
	}
}

func nilOrPanic(err error, a ...interface{}) {
	assert(err == nil, append(a, err))
}

func TestLocalServer(*testing.T) {
	s, err := NewLocalServer(net.TCPAddr{
		IP:   []byte{127, 0, 0, 1},
		Port: 80,
	}, "example.com", "no-secret", "", "", 10*time.Minute)
	nilOrPanic(err)

	path := ""
	url, detach := s.AttachHook(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("Hello World"))
		path = r.URL.Path
	}))

	// Test that we can do GET requests
	r, err := http.NewRequest("GET", url, new(bytes.Buffer))
	w := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(w, r)
	assert(w.Code == 200, "Wrong response")
	assert(path == "/", "Wrong path")

	// Test with different suffix
	r, err = http.NewRequest("GET", url+"myfile", new(bytes.Buffer))
	nilOrPanic(err)
	w = httptest.NewRecorder()
	s.server.Handler.ServeHTTP(w, r)
	assert(w.Code == 200, "Wrong response")
	assert(path == "/myfile", "Wrong path")

	url, detach = s.AttachHook(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("Hello World"))
	}))

	// Call in parallel for race detector
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		r, err := http.NewRequest("GET", url, new(bytes.Buffer))
		nilOrPanic(err)
		w := httptest.NewRecorder()
		s.server.Handler.ServeHTTP(w, r)
		assert(w.Code == 200, "Wrong response")
		wg.Done()
	}()
	go func() {
		r, err := http.NewRequest("POST", url, new(bytes.Buffer))
		nilOrPanic(err)
		w := httptest.NewRecorder()
		s.server.Handler.ServeHTTP(w, r)
		assert(w.Code == 200, "Wrong response")
		wg.Done()
	}()
	wg.Wait()

	// Test wrong id
	badurl := url[:len(url)-4] + "333/"
	r, err = http.NewRequest("GET", badurl, new(bytes.Buffer))
	nilOrPanic(err)
	w = httptest.NewRecorder()
	s.server.Handler.ServeHTTP(w, r)
	assert(w.Code != 200, "Wrong response")

	// Test id too short
	shorturl := url[:len(url)-4] + "/"
	r, err = http.NewRequest("GET", shorturl, new(bytes.Buffer))
	nilOrPanic(err)
	w = httptest.NewRecorder()
	s.server.Handler.ServeHTTP(w, r)
	assert(w.Code != 200, "Wrong response")

	detach()
	r, err = http.NewRequest("GET", url, new(bytes.Buffer))
	nilOrPanic(err)
	w = httptest.NewRecorder()
	s.server.Handler.ServeHTTP(w, r)
	assert(w.Code == 404, "Expected 404")
}
