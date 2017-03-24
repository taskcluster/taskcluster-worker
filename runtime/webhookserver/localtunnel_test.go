// +build localtunnel

package webhookserver

import (
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestLocalTunnel(*testing.T) {
	s, err := NewLocalTunnel("")
	defer s.Stop()
	time.Sleep(500 * time.Millisecond) // give the service time...
	nilOrPanic(err)

	path := ""
	url, _ := s.AttachHook(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("Hello World"))
		path = r.URL.Path
	}))

	// Test that we can do GET requests
	r, err := http.Get(url)
	nilOrPanic(err)
	assert(r.StatusCode == 200, "Wrong response")
	assert(path == "/", "Wrong path")

	// Test with different suffix
	r, err = http.Get(url + "myfile")
	nilOrPanic(err)
	assert(r.StatusCode == 200, "Wrong response")
	assert(path == "/myfile", "Wrong path")

	url, detach := s.AttachHook(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("Hello World"))
	}))

	// Call in parallel for race detector
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		res, rerr := http.Get(url)
		nilOrPanic(rerr)
		assert(res.StatusCode == 200, "Wrong response")
		wg.Done()
	}()
	go func() {
		res, rerr := http.Post(url, "", nil)
		nilOrPanic(rerr)
		assert(res.StatusCode == 200, "Wrong response")
		wg.Done()
	}()
	wg.Wait()

	// Test wrong id
	badurl := url[:len(url)-4] + "333/"
	r, err = http.Get(badurl)
	nilOrPanic(err)
	assert(r.StatusCode != 200, "Wrong response")

	// Test id too short
	shorturl := url[:len(url)-4] + "/"
	r, err = http.Get(shorturl)
	nilOrPanic(err)
	assert(r.StatusCode != 200, "Wrong response")

	detach()
	r, err = http.Get(url)
	nilOrPanic(err)
	assert(r.StatusCode == 404, "Expected 404")
}

func TestLocalTunnelStop(*testing.T) {
	s, err := NewLocalTunnel("")
	time.Sleep(500 * time.Millisecond) // give the service some time...
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
