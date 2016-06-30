package qemurun

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"time"
)

// StartVNCViewer will start a "vncviewer" and connect it to "socket".
// Stopping when done is closed.
func StartVNCViewer(socket string, done <-chan struct{}) {
	// Poll for socket
	for {
		_, err := os.Stat(socket)
		if !os.IsNotExist(err) {
			break
		}
		select {
		case <-done:
			return // abort
		default:
		}
		time.Sleep(200 * time.Millisecond)
	}

	// Listen on localhost for connections
	listener, err := net.Listen("tcp", "127.0.0.1:59007")
	if err != nil {
		fmt.Println("Failed to listen on PORT 59007, error: ", err)
	}

	// Proxy vnc connections to the unix domain socket
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				break
			}
			c, err := net.Dial("unix", socket)
			if err != nil {
				conn.Close()
			} else {
				connect(conn, c)
			}
		}
	}()

	// Launch vinagre
	cmd := exec.Command("vinagre", "localhost:59007")
	if err := cmd.Start(); err != nil {
		fmt.Println("Failed to start 'vinagre', error: ", err)
	}

	// When we're told we're done we close the listener and kill vinagre
	<-done
	listener.Close()
	cmd.Process.Kill()
}

func connect(c1, c2 net.Conn) {
	// Connect the two connetions and make sure we always close
	go func() {
		io.Copy(c1, c2)
		c1.Close()
		c2.Close()
	}()
	go func() {
		io.Copy(c2, c1)
		c1.Close()
		c2.Close()
	}()
}
