// +build qemu

package network

import "testing"

func TestTAPCreateDestroy(t *testing.T) {
	err := createTAPDevice("ttap1")
	if err != nil {
		panic(err)
	}
	err = destroyTAPDevice("ttap1")
	if err != nil {
		panic(err)
	}
}
