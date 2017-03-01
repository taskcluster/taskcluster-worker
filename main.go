// Package main hosts the main function for taskcluter-worker.
//
// The code is structured in 4 kinds of extension registries. The
// commands, config, engines and plugins packages each define interfaces and
// registries where implementations of these interfaces can be registered.
//
// To add a new command to taskcluster-worker you must create new sub-package of
// commands/ which implements and registers commands.CommandProvider with the
// commands.Register(name, provider) method. The same pattern is followed for
// implementation of config transformers, engines, and plugins.
//
// All the sub-packages are then imported here, which ensure that they'll all
// be included in the respective extension registries. Exceptions to this
// pattern is the runtime and worker packages. The runtime package and its
// sub-packages implements generic abstractions and utilities to be used by all
// other packages. The worker package implements task execution flow to be used
// by commands.
package main

import "github.com/taskcluster/taskcluster-worker/commands"

// Import all sub-packages from commands/, config/, engines/ and plugins/
// as they will register themselves using extension registries.
import (
	_ "github.com/taskcluster/taskcluster-worker/commands/daemon"
	_ "github.com/taskcluster/taskcluster-worker/commands/help"
	_ "github.com/taskcluster/taskcluster-worker/commands/qemu-build"
	_ "github.com/taskcluster/taskcluster-worker/commands/qemu-guest-tools"
	_ "github.com/taskcluster/taskcluster-worker/commands/qemu-run"
	_ "github.com/taskcluster/taskcluster-worker/commands/schema"
	_ "github.com/taskcluster/taskcluster-worker/commands/shell"
	_ "github.com/taskcluster/taskcluster-worker/commands/shell-server"
	_ "github.com/taskcluster/taskcluster-worker/commands/work"
	_ "github.com/taskcluster/taskcluster-worker/config/abs"
	_ "github.com/taskcluster/taskcluster-worker/config/configtest"
	_ "github.com/taskcluster/taskcluster-worker/config/env"
	_ "github.com/taskcluster/taskcluster-worker/config/packet"
	_ "github.com/taskcluster/taskcluster-worker/config/secrets"
	_ "github.com/taskcluster/taskcluster-worker/engines/enginetest"
	_ "github.com/taskcluster/taskcluster-worker/engines/mock"
	_ "github.com/taskcluster/taskcluster-worker/engines/native"
	_ "github.com/taskcluster/taskcluster-worker/engines/qemu"
	_ "github.com/taskcluster/taskcluster-worker/engines/script"
	_ "github.com/taskcluster/taskcluster-worker/plugins/artifacts"
	_ "github.com/taskcluster/taskcluster-worker/plugins/env"
	_ "github.com/taskcluster/taskcluster-worker/plugins/interactive"
	_ "github.com/taskcluster/taskcluster-worker/plugins/livelog"
	_ "github.com/taskcluster/taskcluster-worker/plugins/maxruntime"
	_ "github.com/taskcluster/taskcluster-worker/plugins/plugintest"
	_ "github.com/taskcluster/taskcluster-worker/plugins/reboot"
	_ "github.com/taskcluster/taskcluster-worker/plugins/success"
)

func main() {
	commands.Run(nil)
}
