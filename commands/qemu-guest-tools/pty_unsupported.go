//+build !linux,!darwin,!dragonfly,!freebsd,!netbsd,!openbsd

package qemuguesttools

import (
	"os/exec"

	"github.com/taskcluster/taskcluster-worker/plugins/interactive"
)

func pipePty(cmd *exec.Cmd, handler *interactive.ShellHandler) error {
	// On unsupported platform was just fallback to piping a command
	return pipeCommand(cmd, handler)
}
