// +build docker

package network

import (
	"fmt"
	"net/http"
	"os"
	"testing"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/taskcluster/taskcluster-worker/runtime/mocks"
)

const dockerSocket = "/var/run/docker.sock"

func TestNetwork(t *testing.T) {
	// Skip if we don't have a docker socket
	info, err := os.Stat(dockerSocket)
	if err != nil || info.Mode()&os.ModeSocket == 0 {
		t.Skip("didn't find docker socket at:", dockerSocket)
	}

	// Create docker client
	client, err := docker.NewClient("unix://" + dockerSocket)
	require.NoError(t, err, "failed to create docker client")

	var n *Network
	// Test creation of network
	debug("New()")
	n, err = New(client, mocks.NewMockMonitor(false))
	require.NoError(t, err, "failed to create *Network")

	// Set network handler
	n.SetHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Test lack of access from wrong IP
	debug("http.Get from gateway (error expected in debug log)")
	r, rerr := http.Get(fmt.Sprintf("http://%s", n.Gateway()))
	require.NoError(t, rerr, "failed to request gateway")
	defer r.Body.Close()
	assert.Equal(t, http.StatusInternalServerError, r.StatusCode,
		"expected 500 because we're not requesting from a docker container")

	// Test that we can cleanup
	debug("Dispose()")
	err = n.Dispose()
	assert.NoError(t, err, "failed to dispose network")
}
