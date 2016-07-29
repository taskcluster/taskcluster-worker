package webhookserver

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/stateless-dns-go/hostname"
)

// LocalServer is a WebHookServer implemenation that exposes webhooks on a
// local port directly exposed to the internet.
type LocalServer struct {
	m      sync.RWMutex
	server http.Server
	hooks  map[string]http.Handler
	url    string
}

// NewLocalServer creates a WebHookServer that listens on localhost and
// uses stateless-dns-server to obtain a hostname.
func NewLocalServer(
	publicAddress net.TCPAddr,
	subdomain, dnsSecret, tlsCert, tlsKey string,
	expiration time.Duration,
) (*LocalServer, error) {
	s := LocalServer{
		hooks: make(map[string]http.Handler),
	}

	// Set port for server to listen on
	s.server.Addr = fmt.Sprintf(":%d", publicAddress.Port)

	// Setup server handler
	s.server.Handler = http.HandlerFunc(s.handle)

	// Setup server TLS configuration
	if tlsCert != "" && tlsKey != "" {
		cert, err := tls.X509KeyPair(
			[]byte(tlsCert),
			[]byte(tlsKey),
		)
		if err != nil {
			return nil, err
		}
		s.server.TLSConfig = &tls.Config{
			NextProtos:   []string{"http/1.1"},
			Certificates: []tls.Certificate{cert},
		}
	}

	// Construct hostname (using stateless-dns-go)
	host := hostname.New(
		publicAddress.IP,
		subdomain,
		time.Now().Add(expiration),
		dnsSecret,
	)

	// Construct URL
	proto := "http"
	port := ""
	if s.server.TLSConfig != nil {
		proto = "https"
		if publicAddress.Port != 443 {
			port = fmt.Sprintf(":%d", publicAddress.Port)
		}
	} else {
		if publicAddress.Port != 80 {
			port = fmt.Sprintf(":%d", publicAddress.Port)
		}
	}
	s.url = proto + "://" + host + port + "/"

	return &s, nil
}

// ListenAndServe starts the local server listening
func (s *LocalServer) ListenAndServe() error {
	if s.server.TLSConfig != nil {
		return s.server.ListenAndServeTLS("", "")
	}
	return s.server.ListenAndServe()
}

func (s *LocalServer) handle(w http.ResponseWriter, r *http.Request) {
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
func (s *LocalServer) AttachHook(handler http.Handler) (url string, detach func()) {
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
