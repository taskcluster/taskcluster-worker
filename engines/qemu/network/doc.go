// Package network contains scripts and abstractions for setting up TAP-device
// based networks for a set of QEMU virtual machines.
//
// Each virtual machine will get a TAP device, represented as a network, the
// TAP device will automatically get an IP address and DNS server using DHCP.
// The DNS server will resolve the "taskcluster" domains to the meta-data IP
// address. Request to the meta-data IP will be forwarded to the handler
// registered for the network instance.
//
// This package uses iptables to lock down network and ensure that the virtual
// machine attached to a TAP device can't contact the meta-data handler of
// another virtual machine.
package network

import "github.com/taskcluster/taskcluster-worker/runtime"

var debug = runtime.Debug("network")
