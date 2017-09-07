package system

import "io"

// ProcessOptions are the arguments given for StartProcess.
// This structure is platform independent.
type ProcessOptions struct {
	Arguments     []string          // Command and arguments, default to shell
	Environment   map[string]string // Environment variables
	WorkingFolder string            // Working directory, if not HOME
	Owner         *User             // User to run process as, nil to use current
	Groups        []*Group          // Groups to run the process with, only if user is set
	Stdin         io.ReadCloser     // Stream with stdin, or nil if nothing
	Stdout        io.WriteCloser    // Stream for stdout
	Stderr        io.WriteCloser    // Stream for stderr, or nil if using stdout
	TTY           bool              // Start as TTY, if supported, ignores stderr
}
