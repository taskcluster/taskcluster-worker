// Package extpoints provides extention points plugins can satisfy and register
// themselves as furfulling.
package extpoints

// CommandProvider provides a command that can be executed.
type CommandProvider interface {
	Execute() string
}
