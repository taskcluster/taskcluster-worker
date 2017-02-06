package system

// Process is a representation of a system process.
type Process struct {
	// TODO: implement this
}

// Wait for process to terminate, returns true, if exited zero.
func (p *Process) Wait() bool {
	panic("Not implemented")
}

// Kill the process
func (p *Process) Kill() {
	panic("Not implemented")
}

// SetSize of the TTY, if running as TTY or do nothing.
func (p *Process) SetSize(columns, rows uint16) {
	// Do nothing, as this is never supported on windows
}

// StartProcess starts a new process with given arguments, environment variables,
// and current working folder, running as given user.
//
// Returns an human readable error explaining why the sub-process couldn't start
// if not successful.
func StartProcess(options ProcessOptions) (*Process, error) {
	panic("Not implemented")
}

// KillByOwner will kill all process with the given owner.
func KillByOwner(user *User) error {
	panic("Not implemented")
}

// KillChildren will kill all process that are child of the given process.
func KillChildren(process *Process) error {
	panic("Not implemented")
}
