package network

import (
	"fmt"
	"os/exec"
)

// script executes a sequence of commands and returns an error if anything in
// script exited non-zero.
func script(script [][]string) error {
	for _, args := range script {
		c := exec.Command(args[0], args[1:]...)
		c.Stdin = nil
		c.Stderr = nil
		c.Stdout = nil
		err := c.Run()
		if err != nil {
			return fmt.Errorf("Command failed: %v, error: %s", args, err)
		}
	}
	return nil
}
