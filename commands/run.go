// Package commands exposes a run method for main() to call
package commands

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/docopt/docopt-go"
	"github.com/taskcluster/taskcluster-worker/commands/extpoints"
)

// Run will parse command line arguments and run available commands.
func Run(argv []string) {
	// Construct usage string
	usage := "usage: taskcluster-worker <command> [<args>...]\n"
	usage += "\n"
	usage += "Commands available:\n"
	providers := extpoints.CommandProviders.All()
	names := extpoints.CommandProviders.Names()
	sort.Strings(names)
	maxNameLength := 0
	for _, name := range names {
		if len(name) > maxNameLength {
			maxNameLength = len(name)
		}
	}
	for _, name := range names {
		provider := providers[name]
		usage += "\n    " + pad(name, maxNameLength) + " " + provider.Summary()
	}
	usage += "\n"

	// Parse arguments
	arguments, _ := docopt.Parse(usage, argv, true, "taskcluster-worker", true)
	cmd := arguments["<command>"].(string)

	// Find command provider
	provider := providers[cmd]
	if provider == nil {
		fmt.Println("Unknown command: ", cmd)
		fmt.Print(usage)
		os.Exit(1)
	}

	// Parse args for command provider
	subArguments, _ := docopt.Parse(
		provider.Usage(), append([]string{cmd}, arguments["<args>"].([]string)...),
		true, "taskcluster-worker", false,
	)
	// Execute provider with parsed args
	provider.Execute(subArguments)
}

func pad(s string, length int) string {
	p := length - len(s)
	if p < 0 {
		p = 0
	}
	return s + strings.Repeat(" ", p)
}
