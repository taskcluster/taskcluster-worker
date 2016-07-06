// +build qemu

package network

import (
	"fmt"
	"testing"
)

func TestNetworkCreateDestroy(t *testing.T) {
	fmt.Println(" - Creating network pool")
	p, err := NewPool(3)
	nilOrPanic(err, "Failed to create pool")

	fmt.Println(" - Destroying network pool")
	err = p.Dispose()
	nilOrPanic(err, "Failed to dispose networks.")

	fmt.Println(" - Network pool destroyed")
}
