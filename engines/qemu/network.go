package qemu

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/taskcluster/taskcluster-worker/runtime/atomics"

	"gopkg.in/tylerb/graceful.v1"
)

const metaDataIP = "169.254.169.254"

var remoteAddrPattern = regexp.MustCompile("^(192\\.168\\.\\d{1,3})\\.\\d{1,3}:\\d{1,5}$")

// networkPool manages a static set of networks.
type networkPool struct {
	m           sync.Mutex
	idle        []*network
	networks    map[string]*network // mapping from ip-prefix to network
	server      *graceful.Server
	dnsmasq     *exec.Cmd
	dnsmasqKill atomics.Bool
}

// network represents a network with a meta-data service, that only attached
// VMs can talk to (assuming they are attached correctly).
type network struct {
	tapDevice string
	ipPrefix  string // 192.168.xxx (subnet without the last ".0")
	m         sync.RWMutex
	handler   http.Handler
	pool      *networkPool
}

// script executes a sequence of commands and returns an error if anything in
// script exited non-zero.
func script(script [][]string) error {
	for _, args := range script {
		c := exec.Command(args[0], args[1:]...)
		c.Stdin = nil
		c.Stderr = nil
		c.Stdout = nil
		err := c.Run()
		if err != nil {
			return fmt.Errorf("Command failed: %v, error: %s", args, err)
		}
	}
	return nil
}

// newNetworkPool defines the virtual networks and returns networkPool.
// This should be called before the worker starts operating, we don't wish to
// dynamically reconfigure networks at runtime.
func newNetworkPool(number int) *networkPool {
	p := &networkPool{
		networks: make(map[string]*network),
	}

	// Maybe we could split the address space further someday in the future
	if number > 100 {
		panic("Can't create more than 100 networks")
	}

	// Create a number of networks
	for i := 0; i < number; i++ {
		// Construct the network object
		n := createNetwork(i, p)
		p.idle = append(p.idle, n)
		p.networks[n.ipPrefix] = n
	}

	// Enable IPv4 forwarding
	err := script([][]string{
		{"/sbin/sysctl", "-w", "net.ipv4.ip_forward=1"},
	})
	if err != nil {
		panic(fmt.Sprint("Failed to enable ipv4 forwarding: ", err))
	}

	// Create dnsmasq configuration
	dnsmasqConfig := `
    strict-order
    bind-interfaces
    except-interface=lo
    conf-file=""
    dhcp-no-override
    host-record=taskcluster,` + metaDataIP + `
    keep-in-foreground
    bogus-priv
    domain-needed` // Consider adding "no-ping"
	for _, n := range p.networks {
		dnsmasqConfig += `
      interface=` + n.tapDevice + `
      dhcp-range=tag:` + n.tapDevice + `,` + n.ipPrefix + `.2,` + n.ipPrefix + `.254,255.255.255.0,20m
      dhcp-option=tag:` + n.tapDevice + `,option:router,` + n.ipPrefix + `.1`
	}

	// Start dnsmasq
	p.dnsmasq = exec.Command("/sbin/dnsmasq", "--conf-file=-")
	p.dnsmasq.Stdin = bytes.NewBufferString(dnsmasqConfig)
	p.dnsmasq.Stderr = nil
	p.dnsmasq.Stdout = nil
	err = p.dnsmasq.Start()
	if err != nil {
		panic(fmt.Sprint("Failed to start dnsmasq, error: ", err))
	}
	// Monitor dnsmasq and panic if it crashes unexpectedly
	go (func(p *networkPool) {
		err := p.dnsmasq.Wait()
		// Ignore errors if dnsmasqKill is true, otherwise this is a fatal issue
		if err != nil && !p.dnsmasqKill.Get() {
			// We could probably restart the dnsmasq, as long as we avoid an infinite
			// loop that should be fine. But dnsmasq probably won't crash without a
			// good reason.
			panic(fmt.Sprint("Fatal: dnsmasq died unexpectedly, error: ", err))
		}
	})(p)

	// Add meta-data IP to loopback device
	err = script([][]string{
		{"/sbin/ip", "addr", "add", metaDataIP, "dev", "lo"},
	})
	if err != nil {
		panic(fmt.Sprint("Failed to add: ", metaDataIP, " to the loopback device: ", err))
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
	if len(p.idle) < len(p.networks) {
		panic("networkPool.Dispose() cannot be called while a network is in use")
	}

	// Gracefully stop the server
	p.server.Stop(30 * time.Second)

	// Kills dnsmasq
	p.dnsmasqKill.Set(true) // indicate that error exit is expected
	p.dnsmasq.Process.Kill()
	p.dnsmasq.Wait()

	// Delete all the networks
	errs := []string{}
	for _, network := range p.networks {
		err := network.destroy()
		if err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n"))
	}

	// Remove meta-data IP from loopback device
	err := script([][]string{
		{"/sbin/ip", "addr", "del", metaDataIP, "dev", "lo"},
	})

	return err
}

// ipTableRules returns a list of commands to append rules for tapDevice.
// If delete=false, this returns the commands to delete the rules.
func ipTableRules(tapDevice string, ipPrefix string, delete bool) [][]string {
	subnet := ipPrefix + ".0/24"
	gateway := ipPrefix + ".1"
	prefixCommands := func(prefix []string, rules [][]string) [][]string {
		cmds := [][]string{}
		for _, rule := range rules {
			cmds = append(cmds, append(prefix, rule...))
		}
		return cmds
	}

	ruleAction := "-A"
	chainAction := "-N"
	if delete {
		ruleAction = "-D"
		chainAction = "-X"
	}

	// Create/delete custom chains for this tap device
	chains := prefixCommands([]string{"/sbin/iptables", chainAction}, [][]string{
		{"input_" + tapDevice},
		{"output_" + tapDevice},
		{"fwd_input_" + tapDevice},
		{"fwd_output_" + tapDevice},
	})

	// Rules for jumping to custom chains for this tap device
	rules := prefixCommands([]string{"/sbin/iptables", ruleAction}, [][]string{
		{"INPUT", "-i", tapDevice, "-j", "input_" + tapDevice},
		{"OUTPUT", "-o", tapDevice, "-j", "output_" + tapDevice},
		{"FORWARD", "-i", tapDevice, "-j", "fwd_input_" + tapDevice},
		{"FORWARD", "-o", tapDevice, "-j", "fwd_output_" + tapDevice},
	})

	// Rules for nat from this subnet
	nat := prefixCommands([]string{"/sbin/iptables", "-t", "nat", ruleAction}, [][]string{
		{"POSTROUTING", "-o", "eth0", "-s", subnet, "-j", "MASQUERADE"},
	})

	// Rules for filtering INPUT from this tap device
	inputRules := prefixCommands([]string{"/sbin/iptables", ruleAction, "input_" + tapDevice}, [][]string{
		// Allow requests to meta-data service (from subnet only)
		{"-p", "tcp", "-s", subnet, "-d", metaDataIP, "-m", "tcp", "--dport", "80", "-m", "state", "--state", "NEW,ESTABLISHED", "-j", "ACCEPT"},
		// Allow DNS requests
		{"-p", "tcp", "-s", subnet, "-d", gateway, "-m", "tcp", "--dport", "53", "-m", "state", "--state", "NEW,ESTABLISHED", "-j", "ACCEPT"},
		{"-p", "udp", "-s", subnet, "-d", gateway, "-m", "udp", "--dport", "53", "-m", "state", "--state", "NEW,ESTABLISHED", "-j", "ACCEPT"},
		// Allow DCHP requests
		{"-s", "0.0.0.0", "-d", "255.255.255.255", "-p", "udp", "-m", "udp", "--sport", "68", "--dport", "67", "-j", "ACCEPT"},
		{"-s", subnet, "-d", gateway, "-p", "udp", "-m", "udp", "--sport", "68", "--dport", "67", "-j", "ACCEPT"},
		// Reject all other input (with special case for wrong port on meta-data service)
		{"-s", subnet, "-d", metaDataIP, "-j", "REJECT", "--reject-with", "icmp-port-unreachable"},
		{"-j", "REJECT", "--reject-with", "icmp-host-unreachable"},
	})

	// Rules for filtering OUTPUT to this tap decice
	outputRules := prefixCommands([]string{"/sbin/iptables", ruleAction, "output_" + tapDevice}, [][]string{
		// Allow meta-data replies (to subnet only)
		{"-p", "tcp", "-s", metaDataIP, "-d", subnet, "-m", "tcp", "--sport", "80", "-m", "state", "--state", "ESTABLISHED", "-j", "ACCEPT"},
		// Allow DNS replies from dnsmasq (to subnet only)
		{"-p", "udp", "-s", gateway, "-d", subnet, "-m", "udp", "--sport", "53", "-m", "state", "--state", "ESTABLISHED", "-j", "ACCEPT"},
		{"-p", "tcp", "-s", gateway, "-d", subnet, "-m", "tcp", "--sport", "53", "-m", "state", "--state", "ESTABLISHED", "-j", "ACCEPT"},
		// Allow DHCP replies
		{"-p", "udp", "-s", gateway, "-m", "udp", "--sport", "67", "--dport", "68", "-j", "ACCEPT"},
		// Reject all other output
		{"-j", "REJECT", "--reject-with", "icmp-net-prohibited"},
	})

	// Rules for filtering FORWARD from this tap device
	forwardInputRules := prefixCommands([]string{"/sbin/iptables", ruleAction, "fwd_input_" + tapDevice}, [][]string{
		// Reject out-going from tctap1 to private subnets
		{"-d", "10.0.0.0/8", "-j", "REJECT", "--reject-with", "icmp-net-unreachable"},
		{"-d", "172.16.0.0/12", "-j", "REJECT", "--reject-with", "icmp-net-unreachable"},
		{"-d", "169.254.0.0/16", "-j", "REJECT", "--reject-with", "icmp-net-unreachable"},
		{"-d", "192.168.0.0/16", "-j", "REJECT", "--reject-with", "icmp-net-unreachable"},
		// Allow out-going from tctap1 with correct source subnet
		{"-o", "eth0", "-s", subnet, "-j", "ACCEPT"},
		// Allow tctap1 -> tctap1 withing allowed subnet
		{"-o", "tctap1", "-s", subnet, "-j", "ACCEPT"},
		// Reject all other input for forwarding from tap-device
		{"-j", "REJECT", "--reject-with", "icmp-net-prohibited"},
	})

	// Rules for filtering FORWARD to this tap device
	forwardOutputRules := prefixCommands([]string{"/sbin/iptables", ruleAction, "fwd_output_" + tapDevice}, [][]string{
		// Reject incoming from private subnets to tctap1
		{"-s", "10.0.0.0/8", "-j", "DROP"},
		{"-s", "172.16.0.0/12", "-j", "DROP"},
		{"-s", "169.254.0.0/16", "-j", "DROP"},
		{"-s", "192.168.0.0/16", "-j", "DROP"},
		// Allow incoming from tctap1 with correct destination (if already established)
		{"-i", "eth0", "-d", subnet, "-m", "state", "--state", "RELATED,ESTABLISHED", "-j", "ACCEPT"},
		// Allow tctap1 -> tctap1 withing allowed subnet
		{"-i", "tctap1", "-s", subnet, "-j", "ACCEPT"},
		// Reject all other output from forwarding to tap-device
		{"-j", "DROP"},
	})

	cmds := [][]string{}
	if !delete {
		cmds = append(cmds, nat...)
		cmds = append(cmds, chains...)
		cmds = append(cmds, rules...)
		cmds = append(cmds, inputRules...)
		cmds = append(cmds, outputRules...)
		cmds = append(cmds, forwardOutputRules...)
		cmds = append(cmds, forwardInputRules...)
	} else {
		// Reverse order when deleting, because we can't delete chains that are
		// referenced by a rule
		cmds = append(cmds, forwardInputRules...)
		cmds = append(cmds, forwardOutputRules...)
		cmds = append(cmds, outputRules...)
		cmds = append(cmds, inputRules...)
		cmds = append(cmds, rules...)
		cmds = append(cmds, chains...)
		cmds = append(cmds, nat...)
	}

	return cmds
}

// createNetwork creates a tap device and related ip-tables configuration.
// This does not start dnsmasq, use newNetworkPool() to create a set of
// networks with dnsmasq running.
func createNetwork(index int, parent *networkPool) *network {
	// Each network has a name and an ip-prefix, we use the 192.168.0.0/16
	// subnet starting from 192.168.150.0
	tapDevice := "tctap" + strconv.Itoa(index)
	ipPrefix := "192.168." + strconv.Itoa(index+150)

	err := script([][]string{
		// Create tap device
		{"/sbin/ip", "tuntap", "add", "dev", tapDevice, "mode", "tap"},
		// Assign IP-address to tap device
		{"/sbin/ip", "addr", "add", ipPrefix + ".1", "dev", tapDevice},
		// Activate the link
		{"/sbin/ip", "link", "set", "dev", tapDevice, "up"},
		// Add route for the network subnet, routing it to the tap device
		{"/sbin/ip", "route", "add", ipPrefix + ".0/24", "dev", tapDevice},
	})
	if err != nil {
		panic(fmt.Sprint("Failed to setup tap device: ", tapDevice, ", error: ", err))
	}

	// Create iptables rules and chains
	err = script(ipTableRules(tapDevice, ipPrefix, false))
	if err != nil {
		panic(fmt.Sprint("Failed to setup ip-tables for tap device: ", tapDevice, ", error: ", err))
	}

	// Construct the network object
	return &network{
		tapDevice: tapDevice,
		ipPrefix:  ipPrefix,
		handler:   nil,
		pool:      parent,
	}
}

// destroy deletes the networks tap device and related ip-tables configuration.
func (n *network) destroy() error {
	n.m.Lock()
	defer n.m.Lock()
	if n.tapDevice == "" {
		return errors.New("network.tapDevice is empty, implying the network has been destroyed")
	}

	// Delete iptables rules and chains
	err := script(ipTableRules(n.tapDevice, n.ipPrefix, true))
	if err != nil {
		return fmt.Errorf("Failed to remove ip-tables for tap device: %s, error: %s", n.tapDevice, err)
	}

	err = script([][]string{
		// Remove route for the network subnet
		{"/sbin/ip", "route", "del", n.ipPrefix + ".0/24", "dev", n.tapDevice},
		// Deactivate the link
		{"/sbin/ip", "link", "set", "dev", n.tapDevice, "down"},
		// Unassign IP-address from tap device
		{"/sbin/ip", "addr", "del", n.ipPrefix + ".1", "dev", n.tapDevice},
		// Delete tap device
		{"/sbin/ip", "tuntap", "del", "dev", n.tapDevice, "mode", "tap"},
	})
	if err != nil {
		return fmt.Errorf("Failed to remove tap device: %s, error: %s", n.tapDevice, err)
	}

	// Clear handler and tapDevice
	n.handler = nil
	n.tapDevice = ""

	return nil
}

// Configure takes an http.Handler and returns the tap device.
func (n *network) Configure(handler http.Handler) string {
	// Lock the network
	n.m.Lock()
	defer n.m.Lock()
	n.handler = handler
	if n.tapDevice == "" {
		panic("network.tapDevice is empty, implying the network has been destroyed")
	}
	return n.tapDevice
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
