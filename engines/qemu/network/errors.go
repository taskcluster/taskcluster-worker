package network

import "errors"

// ErrAllNetworksInUse is used to signal that we don't have any more networks
// available and, thus, can't return one.
var ErrAllNetworksInUse = errors.New("All networks in the network.Pool are in use")
