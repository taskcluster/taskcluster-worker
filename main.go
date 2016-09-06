//go:generate go-import-subtree engines/ plugins/ commands/

// Package main hosts the main function for taskcluter-worker.
package main

import "github.com/taskcluster/taskcluster-worker/commands"

func main() {
	commands.Run(nil)
}
