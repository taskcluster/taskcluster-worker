package enginetest

import (
	"io/ioutil"
	"sync"
	"time"

	"github.com/taskcluster/taskcluster-worker/engines"
)

// The ShellTestCase contains information sufficient to test the interactive
// shell provided by a Sandbox
type ShellTestCase struct {
	*EngineProvider
	// Command to pipe to the Shell over stdin
	Command string
	// Result to expect from the Shell on stdout when running Command
	Stdout string
	// Result to expect from the Shell on stderr when running Command
	Stderr string
	// Command to execute that exits the shell false
	BadCommand string
	// Command to execute that sleeps long enough for Terminate() to kill it
	SleepCommand string
	// Payload for the engine that will contain an interactive environment as
	// described above.
	Payload string
}

// TestCommand checks we can run Command in the shell
func (c *ShellTestCase) TestCommand() {
	debug("## TestCommand")
	r := c.newRun()
	defer r.Dispose()
	r.NewSandboxBuilder(c.Payload)
	r.StartSandbox()

	shell, err := r.sandbox.NewShell()
	nilOrPanic(err, "NewShell Failed")

	wg := sync.WaitGroup{}
	wg.Add(3)
	go func() {
		_, err2 := shell.StdinPipe().Write([]byte(c.Command))
		nilOrPanic(err2, "Failed to write command")
		err2 = shell.StdinPipe().Close()
		nilOrPanic(err2, "Failed to close stdin")
		wg.Done()
	}()
	go func() {
		stdout, err2 := ioutil.ReadAll(shell.StdoutPipe())
		nilOrPanic(err2, "Failed to read stdout")
		assert(string(stdout) == c.Stdout, "Wrong stdout result, got: ", string(stdout))
		wg.Done()
	}()
	go func() {
		stderr, err2 := ioutil.ReadAll(shell.StderrPipe())
		nilOrPanic(err2, "Failed to read stderr")
		assert(string(stderr) == c.Stderr, "Wrong stderr result, got: ", string(stderr))
		wg.Done()
	}()

	result, err := shell.Wait()
	nilOrPanic(err, "Failed to run command")
	assert(result, "Shell returns non-successfully")

	err = shell.Abort()
	assert(err == engines.ErrShellTerminated, "Expected ErrShellTerminated!")

	wg.Wait()
}

// TestBadCommand checks we can run BadCommand in the shell
func (c *ShellTestCase) TestBadCommand() {
	debug("## TestBadCommand")
	r := c.newRun()
	defer r.Dispose()
	r.NewSandboxBuilder(c.Payload)
	r.StartSandbox()

	shell, err := r.sandbox.NewShell()
	nilOrPanic(err, "NewShell Failed")

	wg := sync.WaitGroup{}
	wg.Add(3)
	go func() {
		_, err2 := shell.StdinPipe().Write([]byte(c.BadCommand))
		nilOrPanic(err2, "Failed to write command")
		err2 = shell.StdinPipe().Close()
		nilOrPanic(err2, "Failed to close stdin")
		wg.Done()
	}()
	go func() {
		_, err2 := ioutil.ReadAll(shell.StdoutPipe())
		nilOrPanic(err2, "Failed to read stdout")
		wg.Done()
	}()
	go func() {
		_, err2 := ioutil.ReadAll(shell.StderrPipe())
		nilOrPanic(err2, "Failed to read stderr")
		wg.Done()
	}()

	result, err := shell.Wait()
	nilOrPanic(err, "Shell returned, error: ", err)
	assert(!result, "Shell returns successfully, expected BadCommand not to!")
	wg.Wait()
}

// TestAbortSleepCommand checks we can Abort the sleep command
func (c *ShellTestCase) TestAbortSleepCommand() {
	debug("## TestAbortSleepCommand")
	r := c.newRun()
	defer r.Dispose()
	r.NewSandboxBuilder(c.Payload)
	r.StartSandbox()

	shell, err := r.sandbox.NewShell()
	nilOrPanic(err, "NewShell Failed")

	wg := sync.WaitGroup{}
	wg.Add(3)
	go func() {
		_, err2 := shell.StdinPipe().Write([]byte(c.SleepCommand))
		nilOrPanic(err2, "Failed to write command")
		err2 = shell.StdinPipe().Close()
		nilOrPanic(err2, "Failed to close stdin")
		time.Sleep(1 * time.Millisecond)
		err2 = shell.Abort()
		nilOrPanic(err2, "Failed abort the shell")
		wg.Done()
	}()
	go func() {
		_, err2 := ioutil.ReadAll(shell.StdoutPipe())
		nilOrPanic(err2, "Failed to read stdout")
		wg.Done()
	}()
	go func() {
		_, err2 := ioutil.ReadAll(shell.StderrPipe())
		nilOrPanic(err2, "Failed to read stderr")
		wg.Done()
	}()

	result, err := shell.Wait()
	assert(!result, "Shell returns successfully, expected Abort to cause false!")
	assert(err == engines.ErrShellAborted, "Expected ErrShellAborted")
	wg.Wait()
}

// Test runs all tests in parallel
func (c *ShellTestCase) Test() {
	wg := sync.WaitGroup{}
	wg.Add(3)
	go func() { c.TestCommand(); wg.Done() }()
	go func() { c.TestBadCommand(); wg.Done() }()
	go func() { c.TestAbortSleepCommand(); wg.Done() }()
	wg.Wait()
}
