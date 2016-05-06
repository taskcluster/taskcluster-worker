// +build vagrant

package network

import (
	"fmt"
	"testing"
)

func TestNetworkCreateDestroy(t *testing.T) {
	p := NewPool(3)

	fmt.Println("Created network pool")

	err := p.Dispose()
	nilOrPanic(err, "Failed to dispose networks.")
}
