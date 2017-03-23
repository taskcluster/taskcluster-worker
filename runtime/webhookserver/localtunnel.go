package webhookserver

import (
	"net/http"
	"strings"
	"sync"

	"github.com/jonasfj/go-localtunnel"
	"github.com/taskcluster/slugid-go/slugid"
)

// LocalTunnel is a WebHookServer implementation based on localtunnel.me
//
// Useful when testing on localhost, should obviously never be used in
// production due to stability, security and scalability constraints.
type LocalTunnel struct {
	m        sync.RWMutex
	listener *localtunnel.Listener
	hooks    map[string]http.Handler
}

// NewLocalTunnel creates a LocalTunnel
//
// Defaults to localtunnel.me if no baseURL is specified.
func NewLocalTunnel(baseURL string) (*LocalTunnel, error) {
	l, err := localtunnel.Listen(localtunnel.Options{
		BaseURL: baseURL,
	})
	if err != nil {
		return nil, err
	}

	lt := &LocalTunnel{
		listener: l,
		hooks:    make(map[string]http.Handler),
	}
	go http.Serve(l, http.HandlerFunc(lt.handle))
	return lt, nil
}

// Stop will close the localtunnel and break all connections
func (lt *LocalTunnel) Stop() {
	lt.listener.Close()
}

func (lt *LocalTunnel) handle(w http.ResponseWriter, r *http.Request) {
	if len(r.URL.Path) < 24 || r.URL.Path[23] != '/' {
		http.NotFound(w, r)
		return
	}

	// Find the hook
	id := r.URL.Path[1:23]
	lt.m.RLock()
	hook := lt.hooks[id]
	lt.m.RUnlock()

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
func (lt *LocalTunnel) AttachHook(handler http.Handler) (url string, detach func()) {
	lt.m.Lock()
	defer lt.m.Unlock()

	// Add hook
	id := slugid.Nice()
	lt.hooks[id] = handler

	// Create url and detach function
	url = lt.listener.URL()
	if !strings.HasSuffix(url, "/") {
		url += "/"
	}
	url += id + "/"
	detach = func() {
		lt.m.Lock()
		defer lt.m.Unlock()
		delete(lt.hooks, id)
	}
	return
}
