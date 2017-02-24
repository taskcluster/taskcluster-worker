package network

import "testing"

func nilOrFatal(t *testing.T, err error, a ...interface{}) {
	if err != nil {
		t.Fatal(append(a, err)...)
	}
}

func assert(t *testing.T, condition bool, a ...interface{}) {
	if !condition {
		t.Fatal(a...)
	}
}
