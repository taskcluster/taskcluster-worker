package network

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/pkg/errors"
	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
)

// Network wraps a docker network and server listening for requests on the
// gateway IP, used as meta-data service.
type Network struct {
	docker    *docker.Client
	handler   http.Handler
	server    http.Server
	m         sync.RWMutex
	monitor   runtime.Monitor
	networkID string
	gateway   string
	subnet    *net.IPNet
	disposed  atomics.Bool
}

// New creates a new Network
func New(client *docker.Client, monitor runtime.Monitor) (*Network, error) {
	n := &Network{
		docker:  client,
		monitor: monitor,
	}
	n.server.Handler = http.HandlerFunc(n.handleRequest)

	// Create an isolated network
	network, err := n.docker.CreateNetwork(docker.CreateNetworkOptions{
		Name:     slugid.Nice(),
		Driver:   "bridge",
		Internal: false,
	})
	if err != nil {
		return nil, errors.Wrap(err, "docker.CreateNetwork failed")
	}
	n.networkID = network.ID

	// Inspect network to get the IP assigned
	network, err = n.docker.NetworkInfo(n.networkID)
	if err != nil {
		return nil, errors.Wrap(err, "docker.NetworkInfo failed")
	}
	// The gateway is the IP of the host machine, we to insert that has hostname
	// "taskcluster" so that proxy calls gets forwarded
	n.gateway = network.IPAM.Config[0].Gateway

	// Find subnet, so we can ignore requests from other subnets
	subnet := network.IPAM.Config[0].Subnet
	_, n.subnet, err = net.ParseCIDR(subnet)
	for err != nil {
		return nil, errors.Wrapf(err, "net.ParseCIDR failed on '%s'", subnet)
	}

	// Listen for requests on gateway IP
	l, err := net.Listen("tcp", fmt.Sprintf("%s:80", n.gateway))
	if err != nil {
		return nil, errors.Wrapf(err, "net.Listen failed for %s:80", n.gateway)
	}
	go func() {
		serr := n.server.Serve(l)
		if serr != nil && n.disposed.Get() {
			n.monitor.ReportError(errors.Wrap(err, "Network.server.Serve failed"))
		}
	}()

	return n, nil
}

// SetHandler allows setting an http.Handler for requests to the gateway IP
// on port 80. This maybe called multiple times to overwrite the handler and
// it maybe be called with nil to unset the handler.
func (n *Network) SetHandler(handler http.Handler) {
	n.m.Lock()
	defer n.m.Unlock()

	n.handler = handler
}

// NetworkID returns the docker network identifier for this network
func (n *Network) NetworkID() string {
	return n.networkID
}

// Gateway returns the IP hosting the http.Handler given in New
func (n *Network) Gateway() string {
	return n.gateway
}

func (n *Network) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Check that incoming request comes from the subnet associated with the
	// docker network we're wrapping
	ip, err := parseRemoteAddr(r)
	if err != nil {
		n.monitor.ReportError(err, "failed to parse r.RemoteAddr, potential security implications!")
		w.WriteHeader(http.StatusInternalServerError) // Respond with minimal information
		return
	}

	// if request is from another IP, then this could be an attack of some sort
	if !n.subnet.Contains(ip) {
		n.monitor.ReportError(err, "ip for request to container host is outside container IP subnet, this is likely an attack!")
		w.WriteHeader(http.StatusInternalServerError) // Respond with minimal information
		return
	}

	// Get handler without holding the lock more than absolutely necessary
	n.m.RLock()
	handler := n.handler
	n.m.RUnlock()

	// If no handle is set we report a warning
	if handler == nil {
		n.monitor.ReportWarning(errors.New("handler not set in network.HandleRequest"), "probably this is a bug or "+
			"race condition where the handler wasn't set, or a security issue "+
			"where a container outlived it's permitted life-cycle",
		)
		w.WriteHeader(http.StatusInternalServerError) // Response with minimal information
		return
	}

	// Forward to whatever handler we were given
	handler.ServeHTTP(w, r)
}

// Dispose releases resources used by the Network and destroys it
func (n *Network) Dispose() error {
	var rerr error // error to return
	// Attmept to stop listening gracefully
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err := n.server.Shutdown(ctx)
	if err != nil && ctx.Err() == nil {
		rerr = errors.Wrap(err, "failed to gracefully shutdown listener on docker network gateway")
	}

	// Stop listening (just in case we timed-out above)
	if err = n.server.Close(); err != nil {
		rerr = errors.Wrap(err, "failed to close server listening on docker network gateway")
	}

	// Remove the network
	if err = n.docker.RemoveNetwork(n.networkID); err != nil {
		rerr = errors.Wrap(err, "failed to remove network")
	}

	// Whatever happens here the last error is the most serious one, so we just
	// return that...
	return rerr
}
