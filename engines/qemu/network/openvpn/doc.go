// Package openvpn provides a wrapper around the openvpn client.
//
// This is aimed at running a VPN client that is configured to expose a
// pre-configured list of routes.
//
// It is primarily intended to connect VMs to services such as KMS, and other
// legacy services that are protected by network access.
package openvpn

import "github.com/taskcluster/taskcluster-worker/runtime/util"

var debug = util.Debug("openvpn")
