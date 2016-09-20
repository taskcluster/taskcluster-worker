package displayclient

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/taskcluster/taskcluster-worker/plugins/interactive/displayconsts"
)

// DisplayClient implements io.Reader, io.Writer and io.Closer for a VNC
// connection over a websocket. Similar to websockify.
type DisplayClient struct {
	mWrite  sync.Mutex // protect ws.WriteMessage
	ws      *websocket.Conn
	mBuffer sync.Mutex // protect buffer
	buffer  []byte
}

// New returns a new display client implementing the ioext.ReadWriteCloser
// interface using a websocket.
//
// The DisplayClient essentially takes care of sending and receiving ping/pongs
// to keep the websocket alive. However, the DisplayClient does read/write
// directly on websocket without any buffering, hence, you must keep calling
// Read() with a non-empty buffer to keep the connection alive.
func New(ws *websocket.Conn) *DisplayClient {
	c := &DisplayClient{
		ws: ws,
	}

	ws.SetReadLimit(displayconsts.DisplayMaxMessageSize)
	ws.SetReadDeadline(time.Now().Add(displayconsts.DisplayPongTimeout))
	ws.SetPongHandler(c.pongHandler)
	go c.sendPings()

	return c
}

func (c *DisplayClient) Read(p []byte) (int, error) {
	// Guard access to c.buffer
	c.mBuffer.Lock()
	defer c.mBuffer.Unlock()

	// Read message until we can populate c.buffer
	for len(c.buffer) == 0 {
		// Read a message
		t, m, err := c.ws.ReadMessage()
		if err != nil {
			return 0, err
		}

		// Skip anything that isn't a binary message
		if t != websocket.BinaryMessage {
			continue
		}

		// Set buffer
		c.buffer = m
	}

	// Copy out from buffer
	n := copy(p, c.buffer)
	c.buffer = c.buffer[n:]

	return n, nil
}

func (c *DisplayClient) Write(p []byte) (int, error) {
	c.mWrite.Lock()
	c.ws.SetWriteDeadline(time.Now().Add(displayconsts.DisplayWriteTimeout))
	err := c.ws.WriteMessage(websocket.BinaryMessage, p)
	c.mWrite.Unlock()

	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// Close will close the underlying websocket and release all resources held by
// the DisplayClient.
func (c *DisplayClient) Close() error {
	// Attempt a graceful close
	c.mWrite.Lock()
	err := c.ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	c.mWrite.Unlock()

	// Always make sure we close properly
	cerr := c.ws.Close()

	// Prefer error sending the close message over any error from closing the
	// websocket.
	if err != nil {
		return err
	}
	return cerr
}

func (c *DisplayClient) sendPings() {
	for {
		// Sleep for ping interval time
		time.Sleep(displayconsts.DisplayPingInterval)

		// Write a ping message, and reset the write deadline
		c.mWrite.Lock()
		c.ws.SetWriteDeadline(time.Now().Add(displayconsts.DisplayWriteTimeout))
		err := c.ws.WriteMessage(websocket.PingMessage, []byte{})
		c.mWrite.Unlock()

		// If there is an error we resolve with internal error
		if err != nil {
			debug("Ping failed, probably the connection was closed, error: %s", err)
			return
		}
	}
}

func (c *DisplayClient) pongHandler(string) error {
	// Reset the read deadline
	c.ws.SetReadDeadline(time.Now().Add(displayconsts.DisplayPongTimeout))
	debug("Received pong, now extending the read-deadline")
	return nil
}
