package network

import (
	"fmt"
	"net"
	"net/http"
)

// parseRemoteAddr returns the IP of the remote IP
func parseRemoteAddr(r *http.Request) (net.IP, error) {
	addr := r.RemoteAddr

	// Remove ":<port>", if present
	host, _, err := net.SplitHostPort(addr)
	if err == nil {
		addr = host
	}

	// Parse IP
	ip := net.ParseIP(addr)
	if ip == nil {
		return nil, fmt.Errorf("invalid remoteAddr: '%s' in parseRemoteAddr", r.RemoteAddr)
	}
	return ip, nil
}
