// +build !windows

package reboot

import (
	"fmt"
	"log"
	"os/exec"
	"syscall"
)

func initiateReboot(command []string) {
	// Send the sigterm to ourselves to start shutdown process earlier; reboot
	// will do this too, but this gives us a few milliseconds head-start.
	// The goal is to avoid to claim a task while reboot command is ongoing
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
		log.Printf("Error calling SIGTERM for self while rebooting: %s\n", err)
		// continue to run the reboot command
	}

	// note, we may have already been terminated here, in which case the
	// reboot will not occur.

	cmd := exec.Command(command[0], command[1:]...)
	if err := cmd.Run(); err != nil {
		// Panic here because we do not want to continue trying to execute
		// jobs in this condition.
		panic(fmt.Sprintf("Could not reboot: %s", err))
	}
}
