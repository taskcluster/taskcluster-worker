package network

import (
	"fmt"
	"testing"
)

func TestNetworkCreateDestroy(t *testing.T) {
	p := NewPool(1)

	fmt.Println("Created network pool")

	err := p.Dispose()
	nilOrPanic(err, "Failed to dispose networks.")
}
