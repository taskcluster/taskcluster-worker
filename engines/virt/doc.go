// Package virt implements an engine that runs virtual machines under libvirt.
//
// To run things with this package you must have 169.254.169.254 as an
// additional address on the loopback device, this can be done with:
//    sudo ip addr add 169.254.169.254/24 scope link dev lo
// Note this can be reverted with:
//    sudo ip addr del 169.254.169.254/24 scope link dev lo
package virt
