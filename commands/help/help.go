// Package help provides the help command.
package help

import (
	"fmt"
	"os"

	"github.com/taskcluster/taskcluster-worker/commands"
)

func init() {
	commands.Register("help", cmd{})
}

type cmd struct{}

func (cmd) Summary() string {
	return "Prints help for a command."
}

func (cmd) Usage() string {
	return "usage: taskcluster-worker help <command>"
}

func (cmd) Execute(arguments map[string]interface{}) bool {
	command := arguments["<command>"].(string)
	provider := commands.Commands()[command]
	if provider == nil {
		fmt.Println("Unknown command: ", command)
		os.Exit(1)
	}
	fmt.Print(provider.Usage())
	return true
}
