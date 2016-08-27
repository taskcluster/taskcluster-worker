package network

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/taskcluster/slugid-go/slugid"
	"gopkg.in/tylerb/graceful.v1"
)

// UserNetwork provides an unsafe network implementation for use when building
// and testing images locally (without root access).
type UserNetwork struct {
	m          sync.Mutex
	socketFile string
	handler    http.Handler
	server     *graceful.Server
	serverDone <-chan struct{}
}

// NewUserNetwork returns a Network implementation using the QEMU user-space
// network stack. This doesn't provide the same level of isolation, but the
// meta-data service should be sufficiently isolated.
func NewUserNetwork(socketFolder string) (*UserNetwork, error) {
	n := &UserNetwork{
		socketFile: filepath.Join(socketFolder, "meta-"+slugid.Nice()+".sock"),
	}
	n.server = &graceful.Server{
		Timeout: 35 * time.Second,
		Server: &http.Server{
			Addr:    metaDataIP + ":80",
			Handler: http.HandlerFunc(n.dispatchRequest),
		},
		NoSignalHandling: true,
	}

	// Start listening (we handle listener error as a special thing)
	listener, err := net.ListenUnix("unix", &net.UnixAddr{
		Name: n.socketFile,
		Net:  "unix",
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to listen on %s error: %s", n.socketFile, err)
	}

	// Start serving
	serverDone := make(chan struct{})
	n.serverDone = serverDone
	go func(n *UserNetwork, done chan<- struct{}) {
		err := n.server.Serve(listener)
		close(done)
		if err != nil {
			panic(fmt.Sprint("Fatal: meta-data service listener failed, error: ", err))
		}
	}(n, serverDone)

	return n, nil
}

func (n *UserNetwork) dispatchRequest(w http.ResponseWriter, r *http.Request) {
	n.m.Lock()
	handler := n.handler
	n.m.Unlock()

	if handler != nil {
		handler.ServeHTTP(w, r)
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

// NetDev returns the argument for the QEMU option -netdev
func (n *UserNetwork) NetDev(ID string) string {
	return "user,id=" + ID + ",net=169.254.0.0/16,guestfwd=tcp:" + metaDataIP + ":80-cmd:netcat -U " + n.socketFile
}

// SetHandler takes an http.Handler to be used for meta-data requests.
func (n *UserNetwork) SetHandler(handler http.Handler) {
	n.m.Lock()
	defer n.m.Unlock()

	if n.socketFile == "" {
		panic("You can't set the handler after Network.Release() have been called")
	}
	n.handler = handler
}

// Release frees all resources used by this network.
func (n *UserNetwork) Release() {
	// Gracefully stop the server
	n.server.Stop(100 * time.Millisecond)
	<-n.serverDone

	// Lock network, ensure we don't release twice
	n.m.Lock()
	defer n.m.Unlock()
	if n.socketFile == "" {
		panic("Can't release a network twice")
	}

	// Remove file and reset state
	os.Remove(n.socketFile)
	n.socketFile = ""
	n.handler = nil
}
