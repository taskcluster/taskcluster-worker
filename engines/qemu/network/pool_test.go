// +build qemu

package network

import (
	"net/http"
	"testing"
)

func TestNetworkCreateDestroy(t *testing.T) {
	for i := 0; i < 2; i++ {
		debug(" - Creating network pool")
		p, err := NewPool(3)
		nilOrPanic(err, "Failed to create pool")

		n1, err := p.Network()
		nilOrPanic(err, "Failed to get network")
		n2, err := p.Network()
		nilOrPanic(err, "Failed to get network")
		n3, err := p.Network()
		nilOrPanic(err, "Failed to get network")
		_, err = p.Network()
		assert(err == ErrAllNetworksInUse, "Expected ErrAllNetworksInUse")

		// Let's make a request to metaDataIP and get a 400 error
		req, err := http.NewRequest(http.MethodGet, "http://"+metaDataIP, nil)
		nilOrPanic(err, "Failed to create http request")
		res, err := http.DefaultClient.Do(req)
		nilOrPanic(err, "Failed to do http request")
		assert(res.StatusCode == http.StatusForbidden, "Expected forbidden")
		res.Body.Close()

		n1.Release()
		n1, err = p.Network()
		nilOrPanic(err, "Failed to get network")

		n1.Release()
		n2.Release()
		n3.Release()

		debug(" - Destroying network pool")
		err = p.Dispose()
		nilOrPanic(err, "Failed to dispose networks.")

		debug(" - Network pool destroyed")
	}
}
