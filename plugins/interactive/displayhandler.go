package interactive

import (
	"io"
	"sync"
	"time"

	"gopkg.in/djherbis/nio.v2"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/websocket"
	"gopkg.in/djherbis/buffer.v1"
)

const displayPongTimeout = 30 * time.Second
const displayPingInterval = 5 * time.Second
const displayWriteTimeout = 10 * time.Second
const displayBufferSize = 32 * 1024

// DisplayHandler handles serving a VNC display socket over a websocket,
// avoiding huge buffers and disposing all resources.
type DisplayHandler struct {
	mWrite  sync.Mutex // guards access to ws.Write
	ws      *websocket.Conn
	display io.ReadWriteCloser
	log     *logrus.Entry
	in      io.ReadCloser
}

// NewDisplayHandler creates a DisplayHandler that connects the websocket to the
// display socket.
func NewDisplayHandler(ws *websocket.Conn, display io.ReadWriteCloser, log *logrus.Entry) *DisplayHandler {
	d := &DisplayHandler{
		ws:      ws,
		display: display,
		log:     log,
		in:      nio.NewReader(display, buffer.New(displayBufferSize)),
	}
	d.ws.SetReadDeadline(time.Now().Add(displayPongTimeout))
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
		time.Sleep(displayPingInterval)

		// Write a ping message, and reset the write deadline
		d.mWrite.Lock()
		debug("Sending ping")
		d.ws.SetWriteDeadline(time.Now().Add(displayWriteTimeout))
		err := d.ws.WriteMessage(websocket.PingMessage, []byte{})
		d.mWrite.Unlock()

		// If there is an error we resolve with internal error
		if err != nil {
			// This is expected when close is called, it's how this for-loop is broken
			if err == websocket.ErrCloseSent {
				d.Abort() // don't log, we probably don't have to call abort() either
				return
			}

			d.log.Error("Failed to send ping, error: ", err)
			d.Abort()
			return
		}
	}
}

func (d *DisplayHandler) pongHandler(string) error {
	d.ws.SetReadDeadline(time.Now().Add(displayPongTimeout))
	return nil
}

func (d *DisplayHandler) sendData() {
	data := make([]byte, displayBufferSize)
	for {
		n, rerr := d.in.Read(data)

		d.mWrite.Lock()
		debug("Sending %d bytes", n)
		d.ws.SetWriteDeadline(time.Now().Add(displayWriteTimeout))
		werr := d.ws.WriteMessage(websocket.BinaryMessage, data[:n])
		d.mWrite.Unlock()

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
				d.log.Error("Failed to read message from websocket, error: ", err)
			}
			d.Abort()
			return
		}

		// Skip anything that isn't a binary message
		if t != websocket.BinaryMessage || len(m) == 0 {
			continue
		}

		_, err = d.display.Write(m)
		if err != nil {
			d.Abort()
			return
		}
	}
}
