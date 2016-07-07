//go:generate go-extpoints
//go:generate go-import-subtree plugins/
package main

import (
	"fmt"
	"os"

	"github.com/jonasfj/go-import-subtree/example/extpoints"
)

func main() {
	// Print list of commands available if no command is given
	if len(os.Args) < 2 {
		fmt.Println("No command given, available commands:")
		for _, cmd := range extpoints.CommandProviders.Names() {
			fmt.Println(cmd)
		}
		os.Exit(1)
	}

	// Let's find the command given
	commandName := os.Args[1]
	commandProvider := extpoints.CommandProviders.Lookup(commandName)
	if commandProvider != nil {
		fmt.Println(commandProvider.Execute())
	} else {
		fmt.Println("Couldn't find command: ", commandName)
	}
}
