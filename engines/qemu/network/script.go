package network

import (
	"bytes"
	"fmt"
	"os/exec"
)

// script executes a sequence of commands and returns an error if anything in
// script exited non-zero.
func script(script [][]string) error {
	stderr := bytes.NewBuffer(nil)
	stdout := bytes.NewBuffer(nil)
	for _, args := range script {
		stderr.Reset()
		stdout.Reset()
		c := exec.Command(args[0], args[1:]...)
		c.Stdin = nil
		c.Stdout = stdout
		c.Stderr = stderr
		err := c.Run()
		if err != nil {
			return fmt.Errorf("Command failed: %v, error: %s, stdout: '%s', stderr: '%s'",
				args, err, stdout.String(), stderr.String())
		}
	}
	return nil
}
