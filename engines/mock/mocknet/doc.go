// Package mocknet implements a net.Listener interface that can reached with
// mocknet.Dial() and establishes connections using net.Pipe()
//
// This is useful for testing things that needs net.Listener and net.Conn
// instances without creating a TCP listener on localhost.
package mocknet

import (
	"sync"

	"github.com/taskcluster/taskcluster-worker/runtime"
)

var debug = runtime.Debug("mocknet")

// mNetworks guards access to networks, which is global list of mock networks.
var (
	mNetworks = sync.Mutex{}
	networks  = make(map[string]*MockListener)
)
