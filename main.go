//go:generate go-import-subtree engines/ plugins/ commands/

package main

import "github.com/taskcluster/taskcluster-worker/commands"

func main() {
	commands.Run(nil)
}
