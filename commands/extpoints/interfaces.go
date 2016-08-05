//go:generate go-extpoints ./

// Package extpoints provides extention points for commands.
package extpoints

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
