// +build !windows

package system

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"os/user"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/process"
	"github.com/taskcluster/taskcluster-worker/plugins/interactive/pty"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

const systemPKill = "/usr/bin/pkill"

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

func pkill(args ...string) error {
	cmd := exec.Command(systemPKill, args...)
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

func killProcesses(root *process.Process) error {
	// Start sending SIGSTOP to avoid that new processes be created while
	// killing children. We don't kill the root process first because
	// if there is another go routine waiting for it, children will
	// be inherited by the *init* process, and we will unable to track
	// them.
	err := root.Suspend()
	if err != nil {
		return err
	}

	children, _ := root.Children()

	// For each child, we recursively kill all their children too.
	var err2 error
	for _, child := range children {
		err = killProcesses(child)
		if err2 == nil {
			err2 = err
		}
	}

	// With all descendents gone, it is time to kill parent
	err = root.Terminate()

	// If we received an error, might be because the process is already gone
	if err == nil {
		// I couldn't find information signals are blocked in stopped processes
		// or not. Let's play safe and send SIGCONT to the process
		root.Resume()

		go func() {
			// Wait 5 seconds after sending SIGTERM, then send SIGKILL.
			// If the process was gone after SIGTERM, Kill() will return
			// an error, which we ignore.
			time.Sleep(5 * time.Second)
			_ = root.Kill()
		}()
	}

	// if there was an error somewhere we return it
	if err != nil {
		return err
	}
	return err2
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

		// If we pass an owner to exec.Start, it will end up calling
		// setgroups (even if we don't have any groups set), a syscall
		// that only root is allowed to execute, causing non-privileged
		// accounts unable to execute process.
		// If the passed owner matches the current user, we set owner to
		// nil to allow non-root accounts to succeed.
		if currentUser.uid == options.Owner.uid {
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
			go func() {
				io.Copy(p.pty, options.Stdin)
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

// KillByOwner will kill all process with the given owner.
func KillByOwner(user *User) error {
	// Create pkill command
	uid := strconv.FormatUint(uint64(user.uid), 10)
	return pkill("-u", uid)
}

// KillProcessTree will kill root and all of its descendents.
func KillProcessTree(root *Process) error {
	proc, err := process.NewProcess(int32(root.cmd.Process.Pid))
	if err != nil {
		return err
	}
	err = killProcesses(proc)
	return err
}
