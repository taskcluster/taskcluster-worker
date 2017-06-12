package system

import (
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"os/user"
	"strconv"
	"sync"

	"github.com/pkg/errors"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

const defaultShell = "cmd.exe"

// Process is a representation of a system process.
type Process struct {
	cmd     *exec.Cmd
	resolve atomics.Once
	sockets sync.WaitGroup
	result  bool
	stdin   io.ReadCloser
	stdout  io.WriteCloser
	stderr  io.WriteCloser
}

// StartProcess starts a new process with given arguments, environment variables,
// and current working folder, running as given user.
//
// Returns an human readable error explaining why the sub-process couldn't start
// if not successful.
func StartProcess(options ProcessOptions) (*Process, error) {
	// Default arguments to system shell
	if len(options.Arguments) == 0 {
		options.Arguments = []string{defaultShell}
	}

	// If WorkingFolder isn't set find home folder of options.Owner (if set)
	// or current user
	if options.WorkingFolder == "" {
		if options.Owner != nil {
			options.WorkingFolder = options.Owner.homeFolder
		} else {
			u, err := user.Current()
			if err != nil {
				panic(fmt.Sprintf("Failed to lookup current user, error: %s", err))
			}
			options.WorkingFolder = u.HomeDir
		}
	}

	if options.Owner != nil {
		currentUser, err := CurrentUser()
		if err != nil {
			return nil, err
		}

		// If the passed owner matches the current user, we set owner to
		// nil to allow non-root accounts to succeed.
		if currentUser.Name() == options.Owner.Name() {
			options.Owner = nil
		}
	}

	// Default stdout to os.DevNul
	if options.Stdout == nil {
		options.Stdout = ioext.WriteNopCloser(ioutil.Discard)
	}

	// Default stderr to stdout
	if options.Stderr == nil {
		options.Stderr = options.Stdout
	}

	// Create process and command
	p := &Process{}
	p.cmd = exec.Command(options.Arguments[0], options.Arguments[1:]...)
	p.cmd.Env = formatEnv(options.Environment)
	p.cmd.Dir = options.WorkingFolder

	// Set owner for the process
	if options.Owner != nil {
		panic("Not implemented: Support for creating processes under a different user is not yet implemented on windows")
	}

	// Start the process
	p.cmd.Stdin = options.Stdin
	p.cmd.Stdout = options.Stdout
	p.cmd.Stderr = options.Stderr
	p.stdin = options.Stdin
	p.stdout = options.Stdout
	p.stderr = options.Stderr

	err := p.cmd.Start()
	if err != nil {
		debug("Failed to start process, error: %s", err)
		return nil, fmt.Errorf("Unable to execute binary, error: %s", err)
	}
	debug("Started process with %v", options.Arguments)

	// Go wait for result
	go p.waitForResult()

	return p, nil
}

func (p *Process) waitForResult() {
	err := p.cmd.Wait()
	debug("Process, cmd.Wait() return: %v", err)

	p.sockets.Wait()

	// Close all connected streams, ignore errors
	if p.stdin != nil {
		p.stdin.Close()
	}
	if p.stdout != nil {
		p.stdout.Close()
	}
	if p.stderr != nil && p.stderr != p.stdout {
		p.stderr.Close()
	}

	// Resolve with result
	p.resolve.Do(func() {
		p.result = err == nil
	})
}

// Wait for process to terminate, returns true, if exited zero.
func (p *Process) Wait() bool {
	p.resolve.Wait()
	return p.result
}

// Kill the process
func (p *Process) Kill() {
	p.cmd.Process.Kill()
}

// SetSize of the TTY, if running as TTY or do nothing.
func (p *Process) SetSize(columns, rows uint16) {
	// Do nothing, as this is never supported on windows
}

// KillByOwner will kill all process with the given owner.
func KillByOwner(user *User) error {
	panic("Not implemented")
}

// KillProcessTree will kill root and all of its descendents.
func KillProcessTree(root *Process) error {
	// See:
	// https://technet.microsoft.com/en-us/library/bb491009.aspx
	// https://ss64.com/nt/taskkill.html
	err := exec.Command(
		`c:\Windows\system32\taskkill.exe`,
		"/F", "/T", "/PID", strconv.Itoa(root.cmd.Process.Pid),
	).Run()
	if err != nil {
		return errors.Wrap(err, "failed to kill process-tree")
	}
	return nil
}
