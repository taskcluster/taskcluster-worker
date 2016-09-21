package webhookserver

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestTestServer(*testing.T) {
	s, err := NewTestServer()
	nilOrPanic(err)

	path := ""
	url, _ := s.AttachHook(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("Hello World"))
		path = r.URL.Path
	}))

	// Test that we can do GET requests
	r, err := http.NewRequest("GET", url, new(bytes.Buffer))
	nilOrPanic(err)
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

	url, detach := s.AttachHook(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("Hello World"))
	}))

	// Call in parallel for race detector
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		req, rerr := http.NewRequest("GET", url, new(bytes.Buffer))
		nilOrPanic(rerr)
		res := httptest.NewRecorder()
		s.server.Handler.ServeHTTP(res, req)
		assert(res.Code == 200, "Wrong response")
		wg.Done()
	}()
	go func() {
		req, rerr := http.NewRequest("POST", url, new(bytes.Buffer))
		nilOrPanic(rerr)
		res := httptest.NewRecorder()
		s.server.Handler.ServeHTTP(res, req)
		assert(res.Code == 200, "Wrong response")
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

func TestTestServerStop(*testing.T) {
	s, err := NewTestServer()
	nilOrPanic(err)

	link, detach := s.AttachHook(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("Hello World"))
	}))

	// Try a request
	r, err := http.NewRequest("GET", link, nil)
	nilOrPanic(err)

	res, err := http.DefaultClient.Do(r)
	nilOrPanic(err)
	assert(res.StatusCode == 200, "Wrong status")

	// Try again after detaching
	detach()
	r, err = http.NewRequest("GET", link, nil)
	nilOrPanic(err)

	res, err = http.DefaultClient.Do(r)
	nilOrPanic(err)
	assert(res.StatusCode == 404, "Wrong status")

	// Stop server, and wait for it to be done
	s.Stop()
}
