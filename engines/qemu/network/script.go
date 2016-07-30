package network

import (
	"bytes"
	"fmt"
	"os/exec"
	"time"
)

// script executes a sequence of commands optionally with retries for each
// command. This methid returns an error if anything failed.
func script(script [][]string, retry bool) error {
	stderr := bytes.NewBuffer(nil)
	stdout := bytes.NewBuffer(nil)
	for _, args := range script {
		// We do up to 90 retries, if retries are allowed, because this is only
		// something we do at start-up. And sometimes ip tuntap reports the device
		// or resource as busy, which given sufficient retries resolves itself.
		// This is mostly relevant during testing where we setup/teardown the
		// network pool a few times. In production we would only do a single setup,
		// but it might be nice that we're as reliable as possible.
		var err error
		for i := 0; i < 90; i++ {
			stderr.Reset()
			stdout.Reset()

			c := exec.Command(args[0], args[1:]...)
			c.Stdin = nil
			c.Stdout = stdout
			c.Stderr = stderr
			err = c.Run()

			if err == nil || !retry {
				break
			}
			debug("Retrying command: %v error: %s, stdout: '%s', stderr: '%s'",
				args, err, stdout.String(), stderr.String())
			time.Sleep(1 * time.Second)
		}
		if err != nil {
			return fmt.Errorf("Command failed: %v, error: %s, stdout: '%s', stderr: '%s'",
				args, err, stdout.String(), stderr.String())
		}
	}
	return nil
}
