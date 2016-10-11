//+build linux darwin dragonfly freebsd netbsd openbsd

package ptysize

import (
	"io/ioutil"
	"os/exec"
	"testing"

	"github.com/kr/pty"
	"github.com/stretchr/testify/require"
)

func TestSet(t *testing.T) {
	cmd := exec.Command("sh")
	f, err := pty.Start(cmd)
	require.NoError(t, err)

	var b []byte
	done := make(chan struct{})
	go func() {
		b, err = ioutil.ReadAll(f)
		close(done)
	}()

	// Set tty size
	Set(f, 50, 100)

	// Print the tty size and exit
	f.WriteString("stty size\nexit 0\n")

	// Wait for termination
	cmd.Wait()
	<-done

	require.Contains(t, string(b), "100 50", "Expected '100 50' in the output")
}
