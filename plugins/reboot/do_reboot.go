// +build !windows

package reboot

import (
	"os/exec"
	"syscall"
)

const rebootCmd = "/sbin/reboot"

func reboot() (err error) {
	// Send the sigterm to ourselves to start shutdown process earlier
	// The goal is to avoid to claim a task while reboot command is ongoing
	if err = syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err == nil {
		cmd := exec.Command(rebootCmd)
		err = cmd.Start()
	}

	return
}
