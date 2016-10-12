//+build linux darwin dragonfly freebsd netbsd openbsd

package pty

import (
	"io/ioutil"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSet(t *testing.T) {
	cmd := exec.Command("sh")
	pty, err := Start(cmd)
	require.NoError(t, err)

	var b []byte
	done := make(chan struct{})
	go func() {
		b, err = ioutil.ReadAll(pty)
		close(done)
	}()

	// Set tty size
	pty.SetSize(50, 100)

	// Print the tty size and exit
	_, err2 := pty.Write([]byte("stty size\nexit 0\n"))
	require.NoError(t, err2)

	// Wait for termination
	cmd.Wait()
	<-done

	require.Contains(t, string(b), "100 50", "Expected '100 50' in the output")
}
