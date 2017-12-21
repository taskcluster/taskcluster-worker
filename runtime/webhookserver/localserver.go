package webhookserver

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"gopkg.in/tylerb/graceful.v1"

	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/stateless-dns-go/hostname"
)

// LocalServer is a WebHookServer implementation that exposes webhooks on a
// local port directly exposed to the internet.
type LocalServer struct {
	m          sync.RWMutex
	server     *graceful.Server
	hooks      map[string]http.Handler
	publicIP   []byte
	publicPort int
	subdomain  string
	dnsSecret  string
	expiration time.Duration
	url        string
	urlOffset  time.Time
}

// NewLocalServer creates a WebHookServer that listens on publicIP, publicPort
// and uses stateless-dns-server to obtain a hostname.
//
// If networkInterface is non-empty and localPort is non-zero then server will
// listen on the localPort for the given networkInterface. This is useful if
// running inside a container.
func NewLocalServer(
	publicIP []byte,
	publicPort int,
	networkInterface string,
	localPort int,
	subdomain, dnsSecret, tlsCert, tlsKey string,
	expiration time.Duration,
) (*LocalServer, error) {
	// 24 hours expiration is usually sane..
	if expiration == 0 {
		expiration = 24 * time.Hour
	}

	s := &LocalServer{
		hooks:      make(map[string]http.Handler),
		publicIP:   publicIP,
		publicPort: publicPort,
		subdomain:  subdomain,
		dnsSecret:  dnsSecret,
		expiration: expiration,
	}

	// Address that we should be listening on
	localAddress := net.TCPAddr{
		IP:   publicIP,
		Port: publicPort,
	}
	if localPort != 0 {
		localAddress.Port = localPort
	}

	// If network interface is specified we listen to that, instead of the public
	// IP address. This is necessary when running inside a docker container where
	// the local network interface isn't assigned to a public IP.
	if networkInterface != "" {
		iface, err := net.InterfaceByName(networkInterface)
		if err != nil {
			return nil, fmt.Errorf(
				"Unable to find interface: %s, error: %s", networkInterface, err,
			)
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return nil, fmt.Errorf(
				"Couldn't list addresses of interface: %s, error: %s", networkInterface, err,
			)
		}
		gotIP := false
		for _, addr := range addrs {
			if a, ok := addr.(*net.IPNet); ok && a.IP.To4() != nil {
				localAddress.IP = a.IP.To4()
				gotIP = true
				break
			}
		}
		if !gotIP {
			return nil, fmt.Errorf("Interface: %s has no IPv4 address", networkInterface)
		}
	}

	// Setup server
	s.server = &graceful.Server{
		Timeout: 35 * time.Second,
		Server: &http.Server{
			Addr:    localAddress.String(),
			Handler: http.HandlerFunc(s.handle),
		},
		NoSignalHandling: true,
	}

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

	return s, nil
}

const dnsExpirationOffset = 30 * time.Minute

func (s *LocalServer) getURL() string {
	if s.url == "" || s.urlOffset.Before(time.Now()) {
		// Offset the URL expiration... so we don't have to create a new hostname
		// for each and every webhook
		s.urlOffset = time.Now().Add(dnsExpirationOffset)

		// Construct hostname (using stateless-dns-go)
		host := hostname.New(
			s.publicIP,
			s.subdomain,
			s.urlOffset.Add(s.expiration),
			s.dnsSecret,
		)

		// Construct URL
		proto := "http"
		port := ""
		if s.server.TLSConfig != nil {
			proto = "https"
			if s.publicPort != 443 {
				port = fmt.Sprintf(":%d", s.publicPort)
			}
		} else {
			if s.publicPort != 80 {
				port = fmt.Sprintf(":%d", s.publicPort)
			}
		}
		s.url = proto + "://" + host + port + "/"
	}

	return s.url
}

// ListenAndServe starts the local server listening
func (s *LocalServer) ListenAndServe() error {
	if s.server.TLSConfig != nil {
		return s.server.ListenAndServeTLSConfig(s.server.TLSConfig)
	}
	return s.server.ListenAndServe()
}

// Stop will stop serving requests
func (s *LocalServer) Stop() {
	s.server.Stop(100 * time.Millisecond)
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
	url = s.getURL() + id + "/"
	detach = func() {
		s.m.Lock()
		defer s.m.Unlock()
		delete(s.hooks, id)
	}
	return
}
