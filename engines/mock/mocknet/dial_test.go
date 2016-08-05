package mocknet

import (
	"io/ioutil"
	"testing"
)

func TestDial(t *testing.T) {
	l, err := Listen("test-net")
	if err != nil {
		panic(err)
	}

	// Accept connections and write hello world back
	go func() {
		c, lerr := l.Accept()
		if lerr != nil {
			if lerr == ErrListenerClosed {
				return
			}
			panic(lerr)
		}
		_, werr := c.Write([]byte("hello-world"))
		if werr != nil {
			panic(werr)
		}
		c.Close()
	}()

	// Dial and read everything
	c, err := Dial("test-net")
	if err != nil {
		panic(err)
	}
	data, err := ioutil.ReadAll(c)
	c.Close()
	if string(data) != "hello-world" {
		panic("Read the wrong thing")
	}
	if err != nil {
		panic(err)
	}

	l.Close()

	// Check that we cleaned up
	mNetworks.Lock()
	if len(networks) != 0 {
		mNetworks.Unlock()
		panic("Networks wasn't cleaned up")
	}
	mNetworks.Unlock()

	// Try to dail again
	_, err = Dial("test-net")
	if err != ErrConnRefused {
		panic("Expected ErrConnRefused")
	}
}
