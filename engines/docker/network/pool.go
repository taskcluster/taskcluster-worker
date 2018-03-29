package network

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/pkg/errors"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

// Pool tracks a set of networks ensuring that they can be re-used
type Pool struct {
	m               sync.Mutex // covering idleNetworks and networksCreated
	docker          *docker.Client
	monitor         runtime.Monitor
	idleNetworks    []*Network
	networksCreated int
	disposed        bool
}

// Handle for a network managed by a Pool
type Handle struct {
	m       sync.Mutex
	pool    *Pool
	network *Network // nil when released
}

// NetworkID returns the docker network identifier for this network
func (h *Handle) NetworkID() string {
	h.m.Lock()
	defer h.m.Unlock()

	// Ensure we haven't released this handle yet
	if h.network == nil {
		panic(errors.New("network.Handle.Gateway() called after network.Handle.NetworkID()"))
	}

	return h.network.NetworkID()
}

// Gateway returns the IP hosting the http.Handler given in New
func (h *Handle) Gateway() string {
	h.m.Lock()
	defer h.m.Unlock()

	// Ensure we haven't released this handle yet
	if h.network == nil {
		panic(errors.New("network.Handle.Gateway() called after network.Handle.Release()"))
	}

	return h.network.Gateway()
}

// Release returns the Network to the Pool
func (h *Handle) Release() {
	h.m.Lock()
	defer h.m.Unlock()

	// If already released then we don't care
	if h.network == nil {
		h.pool.monitor.ReportWarning(errors.New("network.Handle have already been released once"))
		return
	}

	// Lock the Pool
	h.pool.m.Lock()
	defer h.pool.m.Unlock()

	// Ensure that we haven't disposed everything yet
	if h.pool.disposed {
		// We should never attempt to handle this error, it's pretty fatal
		panic(errors.New("network handle can't be released after network.Pool.Dispose()"))
	}

	// Clear the handler
	h.network.SetHandler(nil)
	h.pool.idleNetworks = append(h.pool.idleNetworks, h.network)
	h.network = nil
}

// NewPool returns a new Network pool
func NewPool(client *docker.Client, monitor runtime.Monitor) *Pool {
	return &Pool{
		docker:  client,
		monitor: monitor,
	}
}

// GetNetwork returns a handle for an idle Network
func (p *Pool) GetNetwork(handler http.Handler) (*Handle, error) {
	p.m.Lock()
	defer p.m.Unlock()

	// Ensure that we haven't disposed everything yet
	if p.disposed {
		// We should never attempt to handle this error, it's pretty fatal
		panic(errors.New("network.Pool.GetNetwork called after network.Pool.Dispose()"))
	}

	// Create network if there is no idle networks
	if len(p.idleNetworks) == 0 {
		p.monitor.Infof("creating docker network number: %d", p.networksCreated+1)
		network, err := New(p.docker, p.monitor)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create a new docker network")
		}
		p.networksCreated++
		p.idleNetworks = append(p.idleNetworks, network)
	}

	// Create handle with the last idle network
	h := &Handle{
		pool:    p,
		network: p.idleNetworks[len(p.idleNetworks)-1],
	}
	p.idleNetworks = p.idleNetworks[:len(p.idleNetworks)-1]

	return h, nil
}

// Dispose all Networks held by this Pool.
//
// All Handles must have been released before this method is called.
func (p *Pool) Dispose() error {
	p.m.Lock()
	defer p.m.Unlock()

	// Don't dispose twice, but allow the call (since there is no harm)
	if p.disposed {
		p.monitor.ReportWarning(errors.New("network.Pool have already been disposed once"))
		return nil
	}
	p.disposed = true

	// Warn if not all networks are idle
	if p.networksCreated != len(p.idleNetworks) {
		p.monitor.ReportWarning(fmt.Errorf(
			"found %d idle network in Pool.Dispose() expected %d as was created",
			len(p.idleNetworks), p.networksCreated,
		))
	}

	// Dispose all networks
	var errs []string
	for _, net := range p.idleNetworks {
		if derr := net.Dispose(); derr != nil {
			errs = append(errs, derr.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("Errors while disposing networks: %s", strings.Join(errs, "; "))
	}
	return nil
}
