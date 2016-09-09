package qemurun

import "github.com/taskcluster/taskcluster-worker/commands"

func init() {
	// This command should only be available on linux, so we register it in a file
	// that ends with _linux.go
	commands.Register("qemu-run", cmd{})
}
