package qemubuild

import "github.com/taskcluster/taskcluster-worker/commands/extpoints"

func init() {
	// This command should only be available on linux, so we register it in a file
	// that ends with _linux.go
	extpoints.CommandProviders.Register(cmd{}, "qemu-build")
}
