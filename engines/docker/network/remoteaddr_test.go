package network

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRemoteAddr(t *testing.T) {
	// Setup test server
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, err := parseRemoteAddr(r)
		assert.NoError(t, err, "did not expect error from parseRemoteAddr")
		assert.NotNil(t, ip, "ip should not be nil")
		assert.True(t, len(ip) == net.IPv4len || len(ip) == net.IPv6len,
			"expected len(ip) == net.IPv4len || len(ip) == net.IPv6len")
		w.WriteHeader(http.StatusOK)
		debug("remoteAddr: %s parsed as IP: %s", r.RemoteAddr, ip.String())
	}))
	defer s.Close()

	// Make a request
	r, err := http.Get(s.URL)
	require.NoError(t, err, "request failed")
	r.Body.Close()
}
