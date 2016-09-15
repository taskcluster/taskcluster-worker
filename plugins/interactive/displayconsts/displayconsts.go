package displayconsts

import "time"

const (
	// DisplayHandshakeTimeout is the maximum allowed time for websocket handshake
	DisplayHandshakeTimeout = 30 * time.Second
	// DisplayPingInterval is the time between sending pings
	DisplayPingInterval = 5 * time.Second
	// DisplayWriteTimeout is the maximum time between successful writes
	DisplayWriteTimeout = 10 * time.Second
	// DisplayPongTimeout is the maximum time between successful reads
	DisplayPongTimeout = 30 * time.Second
	// DisplayBufferSize is the internal buffer size, and size of buffer used for
	// sending messages.
	DisplayBufferSize = 32 * 1024
	// DisplayMaxMessageSize is the maximum message size we will read
	DisplayMaxMessageSize = 64 * 1024
)
