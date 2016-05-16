package vm

import "net/http"

// A Network provides a -netdev argument for QEMU, isolation and an HTTP server
// that will handle requests for 169.254.169.254. Ensuring that the requests
// orignates from the virtual machine wit hthe -netdev argument.
type Network interface {
	NetDev(ID string) string         // Argument for the QEMU -netdev option
	SetHandler(handler http.Handler) // Set http.Handler for 169.254.169.254:80
	Release()                        // Release the network after use
}
