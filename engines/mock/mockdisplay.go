package mockengine

import (
	"fmt"
	"image"
	"image/color"
	"io"
	"time"

	"github.com/bradfitz/rfbgo/rfb"
	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-worker/engines/mock/mocknet"
)

const (
	mockDisplayWidth  = 480
	mockDisplayHeight = 640
)

func newMockDisplay() io.ReadWriteCloser {
	// Create listener for randomly generated addr
	addr := slugid.Nice()
	l, err := mocknet.Listen(addr)
	if err != nil {
		// This shouldn't be possible
		panic(fmt.Sprintf("mocknet.Listen failed using random addr, error: %s", err))
	}

	// Create and start server
	s := rfb.NewServer(mockDisplayWidth, mockDisplayHeight)
	go s.Serve(l)

	// Dial up to server
	conn, err := mocknet.Dial(addr)
	if err != nil {
		// This shouldn't happen either
		panic(fmt.Sprintf("mocknet.Dial failed, error: %s", err))
	}

	// Handle display when we get a connection from the server
	go handleDisplay(<-s.Conns) // This works because Conns has a size 16

	// Stop listener, we'll create one for each mock display connection
	l.Close()

	return conn
}

func handleDisplay(c *rfb.Conn) {
	// render display until closed
	closed := make(chan struct{})
	go renderDisplay(c, closed)

	for e := range c.Event {
		debug("VNC: display received event: %+v", e)
	}
	close(closed)
	debug("VNC: display connection closed")
}

func renderDisplay(c *rfb.Conn, closed <-chan struct{}) {
	img := image.NewRGBA(image.Rect(0, 0, mockDisplayWidth, mockDisplayHeight))
	lockedÍmg := rfb.LockableImage{Img: img}

	frame := 0
	for {
		frame++

		// Lock image and paint everything the same color.. Use a gradient of frame
		// counter... So it'll move a bit...
		lockedÍmg.Lock()
		col := color.RGBA{
			R: uint8(frame % 255),
			G: uint8(frame % 255),
			B: uint8(frame % 255),
			A: 255,
		}
		for x := 0; x < 256; x++ {
			for y := 0; y < 256; y++ {
				img.Set(x, y, col)
			}
		}
		lockedÍmg.Unlock()

		select {
		case c.Feed <- &lockedÍmg:
		case <-closed:
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}
