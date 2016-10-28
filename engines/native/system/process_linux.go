package system

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"os/user"
	"sync"
	"syscall"

	"github.com/taskcluster/taskcluster-worker/plugins/interactive/pty"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

const systemPKill = "/usr/bin/pkill"

// test variables
var testGroup = "root"
var testCat = []string{"/bin/cat", "-"}
var testTrue = []string{"/bin/true"}
var testFalse = []string{"/bin/false"}
var testPrintDir = []string{"/bin/pwd"}
var testSleep = []string{"/bin/sleep", "5"}

// Process is a representation of a system process.
type Process struct {
	cmd     *exec.Cmd
	pty     *pty.PTY
	resolve atomics.Once
	sockets sync.WaitGroup
	result  bool
	stdin   io.ReadCloser
	stdout  io.WriteCloser
	stderr  io.WriteCloser
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
	if p.pty != nil {
		p.pty.Close()
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
	if p.pty != nil {
		p.pty.SetSize(columns, rows)
	}
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
		p.cmd.SysProcAttr = &syscall.SysProcAttr{
			Credential: &syscall.Credential{
				Uid: options.Owner.uid,
				Gid: options.Owner.gid,
			},
		}
	}

	// Start the process
	var err error
	if !options.TTY {
		p.cmd.Stdin = options.Stdin
		p.cmd.Stdout = options.Stdout
		p.cmd.Stderr = options.Stderr
		p.stdin = options.Stdin
		p.stdout = options.Stdout
		p.stderr = options.Stderr
		err = p.cmd.Start()
	} else {
		p.pty, err = pty.Start(p.cmd)
		if options.Stdin != nil {
			p.sockets.Add(1)
			go func() {
				io.Copy(p.pty, options.Stdin)
				p.sockets.Done()
				// Kill process when stdin ends (if running as TTY)
				p.Kill()
			}()
		}
		p.sockets.Add(1)
		go func() {
			ioext.CopyAndClose(options.Stdout, p.pty)
			p.sockets.Done()
		}()
	}

	if err != nil {
		debug("Failed to start process, error: %s", err)
		return nil, fmt.Errorf("Unable to execute binary, error: %s", err)
	}
	debug("Started process with %v", options.Arguments)

	// Go wait for result
	go p.waitForResult()

	return p, nil
}

// KillByOwner will kill all process with the given owner.
func KillByOwner(user *User) error {
	// Create pkill command
	cmd := exec.Command(systemPKill, "-u", user.name)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	// Start pkill, error here is pretty fatal
	err := cmd.Start()
	if err != nil {
		panic(fmt.Sprintf("Failed to start pkill, error: %s", err))
	}

	// Wait for pkill to terminate
	err = cmd.Wait()

	// Check error, exitcode 1 means nothing was killed we ignore that
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			if status, ok := e.Sys().(syscall.WaitStatus); ok {
				// Exit status 1 means no processes were killed, that's fine
				if status.ExitStatus() == 1 {
					return nil
				}
			}
			return fmt.Errorf("pkill exited non-zero with output: %s", out.String())
		}
		return fmt.Errorf("pkill error: %s", err)
	}

	return nil
}
