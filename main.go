//go:generate go-import-subtree engines/ plugins/ commands/ config/

// Package main hosts the main function for taskcluter-worker.
package main

import "github.com/taskcluster/taskcluster-worker/commands"

func main() {
	commands.Run(nil)
}
