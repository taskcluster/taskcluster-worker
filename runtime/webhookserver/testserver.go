package webhookserver

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/taskcluster/slugid-go/slugid"

	graceful "gopkg.in/tylerb/graceful.v1"
)

// TestServer is a WebHookServer implementation that exposes webhooks on a
// random port on localhost for testing.
type TestServer struct {
	m      sync.RWMutex
	server *graceful.Server
	hooks  map[string]http.Handler
	url    string
}

// NewTestServer returns a LocalServer running on a random port on localhost,
// this is exclusively for writing tests.
func NewTestServer() (*TestServer, error) {
	s := &TestServer{
		hooks: make(map[string]http.Handler),
	}

	// Setup server
	s.server = &graceful.Server{
		Timeout: 35 * time.Second,
		Server: &http.Server{
			Handler: http.HandlerFunc(s.handle),
		},
		NoSignalHandling: true,
	}
	l, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		return nil, fmt.Errorf("Failed to listen on localhost, error: %s", err)
	}

	s.url = fmt.Sprintf("http://%s/", l.Addr().String())

	go s.server.Serve(l)

	return s, nil
}

// Stop will stop serving requests
func (s *TestServer) Stop() {
	s.server.Stop(100 * time.Millisecond)
}

func (s *TestServer) handle(w http.ResponseWriter, r *http.Request) {
	if len(r.URL.Path) < 24 || r.URL.Path[23] != '/' {
		http.NotFound(w, r)
		return
	}

	// Find the hook
	id := r.URL.Path[1:23]
	s.m.RLock()
	hook := s.hooks[id]
	s.m.RUnlock()

	if hook == nil {
		http.NotFound(w, r)
		return
	}

	r.URL.Path = r.URL.Path[23:]
	r.URL.RawPath = "" // TODO: Implement this if we need it someday

	hook.ServeHTTP(w, r)
}

// AttachHook setups handler such that it gets called when a request arrives
// at the returned url.
func (s *TestServer) AttachHook(handler http.Handler) (url string, detach func()) {
	s.m.Lock()
	defer s.m.Unlock()

	// Add hook
	id := slugid.Nice()
	s.hooks[id] = handler

	// Create url and detach function
	url = s.url + id + "/"
	detach = func() {
		s.m.Lock()
		defer s.m.Unlock()
		delete(s.hooks, id)
	}
	return
}
