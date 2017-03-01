package interactive

import (
	"io"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/taskcluster/taskcluster-worker/plugins/interactive/displayconsts"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

// DisplayHandler handles serving a VNC display socket over a websocket,
// avoiding huge buffers and disposing all resources.
type DisplayHandler struct {
	mWrite  sync.Mutex // guards access to ws.Write
	ws      *websocket.Conn
	display io.ReadWriteCloser
	monitor runtime.Monitor
	in      io.ReadCloser
}

// NewDisplayHandler creates a DisplayHandler that connects the websocket to the
// display socket.
func NewDisplayHandler(ws *websocket.Conn, display io.ReadWriteCloser, monitor runtime.Monitor) *DisplayHandler {
	d := &DisplayHandler{
		ws:      ws,
		display: display,
		monitor: monitor,
		in:      display,
	}
	d.ws.SetReadLimit(displayconsts.DisplayMaxMessageSize)
	d.ws.SetReadDeadline(time.Now().Add(displayconsts.DisplayPongTimeout))
	d.ws.SetPongHandler(d.pongHandler)

	go d.sendPings()
	go d.sendData()
	go d.readMessages()

	return d
}

// Abort the display handler
func (d *DisplayHandler) Abort() {
	d.ws.Close()
	d.display.Close()
}

func (d *DisplayHandler) sendPings() {
	for {
		// Sleep for ping interval time
		time.Sleep(displayconsts.DisplayPingInterval)

		// Write a ping message, and reset the write deadline
		d.mWrite.Lock()
		debug("Sending ping")
		d.ws.SetWriteDeadline(time.Now().Add(displayconsts.DisplayWriteTimeout))
		err := d.ws.WriteMessage(websocket.PingMessage, []byte{})
		d.mWrite.Unlock()

		// If there is an error we resolve with internal error
		if err != nil {
			// This is expected when close is called, it's how this for-loop is broken
			if err == websocket.ErrCloseSent {
				d.Abort() // don't log, we probably don't have to call abort() either
				return
			}

			d.monitor.Error("Failed to send ping, error: ", err)
			d.Abort()
			return
		}
	}
}

func (d *DisplayHandler) pongHandler(string) error {
	d.ws.SetReadDeadline(time.Now().Add(displayconsts.DisplayPongTimeout))
	return nil
}

func (d *DisplayHandler) sendData() {
	data := make([]byte, displayconsts.DisplayBufferSize)
	for {
		n, rerr := d.in.Read(data)
		if rerr != nil {
			debug("Display read error: %s", rerr)
		}

		var werr error
		if n > 0 {
			d.mWrite.Lock()
			debug("Display sending %d bytes", n)
			d.ws.SetWriteDeadline(time.Now().Add(displayconsts.DisplayWriteTimeout))
			werr = d.ws.WriteMessage(websocket.BinaryMessage, data[:n])
			d.mWrite.Unlock()
		}

		if rerr != nil || werr != nil {
			d.Abort()
			return
		}
	}
}

func (d *DisplayHandler) readMessages() {
	for {
		t, m, err := d.ws.ReadMessage()
		if err != nil {
			// This is expected to happen when the loop breaks
			if e, ok := err.(*websocket.CloseError); ok && e.Code == websocket.CloseNormalClosure {
				debug("Websocket closed normally error: ", err)
			} else {
				d.monitor.Error("Failed to read message from websocket, error: ", err)
			}
			d.Abort()
			return
		}

		// Skip anything that isn't a binary message
		if t != websocket.BinaryMessage || len(m) == 0 {
			debug("Display ignoring non-binary message")
			continue
		}

		_, err = d.display.Write(m)
		debug("Display writing %d bytes", len(m))
		if err != nil {
			d.Abort()
			return
		}
	}
}
