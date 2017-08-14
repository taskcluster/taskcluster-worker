package qemurun

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
)

// ExposeVNC will forward requests on given port to given socket until done is
// closed.
func ExposeVNC(socket string, port int, done <-chan struct{}) {
	// Wait for VNC socket to available
	waitForSocket(socket, done)

	// Create a TCP listener
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		panic(fmt.Sprintf("Failed to listen on PORT %d, error: %s", port, err))
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

	// When we're told we're done we close the listener
	<-done
	listener.Close()
}

func waitForSocket(socket string, done <-chan struct{}) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		panic(errors.Wrap(err, "Failed to setup file system monitoring"))
	}
	defer w.Close()

	if err = w.Add(filepath.Dir(socket)); err != nil {
		panic(errors.Wrap(err, "Failed to monitor socket folder"))
	}

	for {
		if _, err := os.Stat(socket); err == nil || !os.IsNotExist(err) {
			return
		}

		select {
		case <-w.Events:
		case <-done:
			return
		}
	}
}

// StartVNCViewer will start a "vncviewer" and connect it to "socket".
// Stopping when done is closed.
func StartVNCViewer(socket string, done <-chan struct{}) {
	waitForSocket(socket, done)
	go ExposeVNC(socket, 59007, done)

	// Launch vinagre
	cmd := exec.Command("vinagre", "localhost:59007")
	if err := cmd.Start(); err != nil {
		fmt.Println("Failed to start 'vinagre', error: ", err)
	}

	// When we're told we're done we close the listener and kill vinagre
	<-done
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
