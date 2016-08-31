package commands

import (
	"fmt"
	"sync"
)

var (
	mCommands = sync.Mutex{}
	commands  = map[string]CommandProvider{}
)

// CommandProvider is implemented by anyone who wishes to provide a command line
// command that the worker should support.
type CommandProvider interface {
	// Summary returns a one-line description of what this command is for.
	Summary() string
	// Usage returns the docopt usage string, used to parse arguments.
	Usage() string
	// Execute is called with parsed docopt result, return true/false if the
	// utility should exit zero or non-zero.
	Execute(args map[string]interface{}) bool
}

// Register will register a CommandProvider, this is intended to be used during
// static initializtion and will panic if name is already in use.
func Register(name string, provider CommandProvider) {
	mCommands.Lock()
	defer mCommands.Unlock()

	if _, ok := commands[name]; !ok {
		panic(fmt.Sprintf("Command name: '%s' is already in use!", name))
	}
	commands[name] = provider
}

// Commands returns a map from name to registered CommandProvider.
func Commands() map[string]CommandProvider {
	mCommands.Lock()
	defer mCommands.Unlock()

	m := map[string]CommandProvider{}
	for name, provider := range commands {
		m[name] = provider
	}
	return m
}
