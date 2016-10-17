package system

import "io"

// Process is a cross-platform abstraction for a sub-process
type Process interface {
	Wait() bool  // Wait for process to terminate, returns true if successful.
	Kill() error // Kill will terminate the process
}

// ProcessOptions are the arguments given for NewProcess.
type ProcessOptions struct {
	Arguments     []string          // Command and arguments, nil to start shell
	Environment   map[string]string // Environment variables
	WorkingFolder string
	Owner         User           // User to run process as, nil to use current
	Stdin         io.ReadCloser  // Stream with stdin, or nil if nothing
	Stdout        io.WriteCloser // Stream for stdout
	Stderr        io.WriteCloser // Stream for stderr, or nil if to use stdout
	TTY           bool           // True, to start as TTY, if supported
}
