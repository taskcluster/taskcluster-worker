package qemu

import "testing"

func TestNetworkCreateDestroy(t *testing.T) {
	p := newNetworkPool(1)

	err := p.Dispose()
	nilOrPanic(err, "Failed to dispose networks.")
}
