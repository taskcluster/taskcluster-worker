package virt

import (
	"encoding/xml"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"time"

	"gopkg.in/tylerb/graceful.v1"

	"github.com/rgbkrk/libvirt-go"
)

const metaDataIP = "169.254.169.254"

// networkPool manages a static set of networks.
type networkPool struct {
	m        sync.Mutex
	idle     []*network
	networks map[string]*network // mapping from ip-prefix to network
	server   *graceful.Server
}

// network represents a network with a meta-data service, that only attached
// VMs can talk to (assuming they are attached correctly).
type network struct {
	name     string
	vnet     libvirt.VirNetwork
	m        sync.RWMutex
	handler  http.Handler
	pool     *networkPool
	ipPrefix string
}

// newNetworkPool defines the virtual networks and returns networkPool.
// This should be called before the worker starts operating, we don't wish to
// dynamically reconfigure networks at runtime.
func newNetworkPool(c *libvirt.VirConnection, number int) *networkPool {
	p := &networkPool{
		networks: make(map[string]*network),
	}

	// Maybe we could split the address space further someday in the future
	if number > 100 {
		panic("Can't create more than 100 networks")
	}

	// Create a number of networks
	for i := 0; i < number; i++ {
		// Each network has a name and an ip-prefix, we use the 192.168.0.0/16
		// subnet starting from 192.168.150.0, as libvirt uses 192.168.122.0 for
		// the default network.
		name := "tc-net-" + strconv.Itoa(i)
		ipPrefix := "192.168." + strconv.Itoa(i+150)

		// Render network definition as XML
		xmlConfig, err := xml.MarshalIndent(Network{
			Name: name,
			Forward: &NetworkForwarder{
				Mode:     "nat",
				NATPorts: &PortRange{Start: 1024, End: 65535},
			},
			Bridge: &NetworkBridge{
				Name:            "virbr" + strconv.Itoa(i+1), // virbr0 is already used
				STP:             "on",
				Delay:           0, // Always zero why would anyone delay something!!
				MACTableManager: "libvirt",
			},
			MAC:   NewMAC(),
			Hosts: []HostRecord{{IP: metaDataIP, Hostname: "taskcluster"}},
			IP: &NetworkAddressing{
				Address: ipPrefix + ".1",
				NetMask: "255.255.255.0",
				IPRange: &IPAddressRange{
					Start: ipPrefix + ".2",
					End:   ipPrefix + ".254",
				},
			},
		}, "", "  ")
		if err != nil {
			panic(fmt.Sprint("Failed render XML for network, error: ", err))
		}

		// Define and create the network
		vnet, err := c.NetworkDefineXML(string(xmlConfig))
		if err != nil {
			panic(fmt.Sprint("Failed to define network, error: ", err, string(xmlConfig)))
		}
		err = vnet.Create()
		if err != nil {
			panic(fmt.Sprint("Failed to create network, error: ", err, string(xmlConfig)))
		}

		// Construct the network object
		n := &network{
			name:     name,
			vnet:     vnet,
			handler:  nil,
			pool:     p,
			ipPrefix: ipPrefix,
		}
		p.idle = append(p.idle, n)
		p.networks[ipPrefix] = n
	}

	// Create the server
	p.server = &graceful.Server{
		Timeout: 30 * time.Second,
		Server: &http.Server{
			Addr:    metaDataIP + ":80",
			Handler: http.HandlerFunc(p.dispatchRequest),
		},
		NoSignalHandling: true,
	}

	// Start listening (we handle listener error as a special thing)
	listener, err := net.Listen("tcp", p.server.Addr)
	if err != nil {
		// If this happens ensure that we have configured the loopback device with:
		// sudo ip addr add 169.254.169.254/24 scope link dev lo
		panic(fmt.Sprint("Failed to listen on ", p.server.Addr, " error: ", err))
	}

	// Start the server
	go (func(p *networkPool) {
		err := p.server.Serve(listener)
		if err != nil {
			// TODO: We could communicate this to all sandboxes and shut them down
			// gracefully. But I honestly doubt this will ever happen, why should it?
			panic(fmt.Sprint("Fatal: meta-data service listener failed, error: ", err))
		}
	})(p)

	return p
}

var remoteAddrPattern = regexp.MustCompile("^(192\\.168\\.\\d{1,3})\\.\\d{1,3}:\\d{1,5}$")

func (p *networkPool) dispatchRequest(w http.ResponseWriter, r *http.Request) {
	// Match remote address to find ipPrefix
	match := remoteAddrPattern.FindStringSubmatch(r.RemoteAddr)
	if len(match) != 2 {
		w.WriteHeader(403)
		return
	}
	ipPrefix := match[1]

	// Find network from the ipPrefix
	n := p.networks[ipPrefix]
	if n == nil {
		w.WriteHeader(403)
		return
	}

	// Read lock the network, so the handler can't be cleared while we do this
	n.m.RLock()
	handler := n.handler
	n.m.RUnlock()

	// Call handler
	handler.ServeHTTP(w, r)
}

// ClaimNetwork returns an unused network, or nil if no network is available.
func (p *networkPool) ClaimNetwork() *network {
	p.m.Lock()
	defer p.m.Unlock()
	if len(p.idle) > 0 {
		n := p.idle[len(p.idle)-1]
		p.idle = p.idle[:len(p.idle)-1]
		return n
	}
	return nil
}

// Dispose deletes all the networks created, should not be called while any of
// networks are in use.
func (p *networkPool) Dispose() error {
	// Gracefully stop the server
	p.server.Stop(30 * time.Second)

	return nil
}

// Configure takes an http.Handler and returns a NetworkInterface.
func (n *network) Configure(handler http.Handler) NetworkInterface {
	// Lock the network
	n.m.Lock()
	defer n.m.Lock()
	n.handler = handler
	return NetworkInterface{
		Type:   "network",
		Source: NetworkSource{Network: n.name},
		MAC:    NewMAC(),
		Filter: &FilterReference{Name: networkFilterRoot, Parameters: []Parameter{
			{Name: "METADATA_IP", Value: metaDataIP},
			{Name: "NETWORK_GATEWAY", Value: n.ipPrefix + ".1"},
			{Name: "NETWORK_MASK", Value: "255.255.255.0"},
		}},
	}
}

// Release removes the handler and returns the network to the networkPool.
// This may not be called while a domain is still using the network.
func (n *network) Release() {
	// Lock the network
	n.m.Lock()
	defer n.m.Lock()
	// Lock the pool
	n.pool.m.Lock()
	defer n.pool.m.Unlock()
	n.handler = nil
	n.pool.idle = append(n.pool.idle, n)
}
