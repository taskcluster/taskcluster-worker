// Package shellconsts contains constants shared between shell server and
// client which is split into different packages to reduce the binary size of
// potential commandline clients.
package shellconsts

import "time"

const (
	// ShellHandshakeTimeout is the maximum allowed time for websocket handshake
	ShellHandshakeTimeout = 30 * time.Second
	// ShellPingInterval is the time between sending pings
	ShellPingInterval = 15 * time.Second
	// ShellWriteTimeout is the maximum time between successful writes
	ShellWriteTimeout = ShellPingInterval * 2
	// ShellPongTimeout is the maximum time between successful reads
	ShellPongTimeout = ShellPingInterval * 3
	// ShellBlockSize is the maximum number of bytes to send in a single block
	ShellBlockSize = 16 * 1024
	// ShellMaxMessageSize is the maximum message size we will read
	ShellMaxMessageSize = ShellBlockSize + 4*1024
	// ShellMaxPendingBytes is the maximum number of bytes allowed in-flight
	ShellMaxPendingBytes = 4 * ShellBlockSize
)
