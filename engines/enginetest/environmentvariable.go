package enginetest

import (
	"sync"

	"github.com/taskcluster/taskcluster-worker/engines"
)

// The EnvVarTestCase contains information sufficient to setting an environment
// variable.
type EnvVarTestCase struct {
	engineProvider
	// Name of engine
	Engine string
	// Valid name for an environment variable.
	VariableName string
	// Invalid environment variable names.
	InvalidVariableNames []string
	// Payload that will print the value of VariableName to the log.
	Payload string
}

// TestPrintVariable checks that variable value can be printed
func (c *EnvVarTestCase) TestPrintVariable() {
	r := c.NewRun(c.Engine)
	defer r.Dispose()
	r.NewSandboxBuilder(c.Payload)
	err := r.sandboxBuilder.SetEnvironmentVariable(c.VariableName, "Hello World")
	nilOrpanic(err, "SetEnvironmentVariable failed")
	assert(r.buildRunSandbox(), "Payload exited unsuccessfully")
	assert(r.GrepLog("Hello World"), "Didn't find variable value in log")
}

// TestVariableNameConflict checks that variable name can't conflict
func (c *EnvVarTestCase) TestVariableNameConflict() {
	r := c.NewRun(c.Engine)
	defer r.Dispose()
	r.NewSandboxBuilder(c.Payload)
	err := r.sandboxBuilder.SetEnvironmentVariable(c.VariableName, "Hello World")
	nilOrpanic(err, "SetEnvironmentVariable failed")
	err = r.sandboxBuilder.SetEnvironmentVariable(c.VariableName, "Hello World2")
	if err != engines.ErrNamingConflict {
		fmtPanic("Expected ErrNamingConflict when declaring: ", c.VariableName, " twice")
	}
}

// TestInvalidVariableNames checks that invalid variables returns correct error
func (c *EnvVarTestCase) TestInvalidVariableNames() {
	r := c.NewRun(c.Engine)
	defer r.Dispose()
	r.NewSandboxBuilder(c.Payload)
	for _, name := range c.InvalidVariableNames {
		err := r.sandboxBuilder.SetEnvironmentVariable(name, "hello world")
		if _, ok := err.(*engines.MalformedPayloadError); ok {
			fmtPanic("Expected MalformedPayloadError from invalid variable name: ", name)
		}
	}
}

// Test runs all tests in parallel
func (c *EnvVarTestCase) Test() {
	c.ensureEngine(c.Engine)
	wg := sync.WaitGroup{}
	wg.Add(3)
	go func() { c.TestPrintVariable(); wg.Done() }()
	go func() { c.TestVariableNameConflict(); wg.Done() }()
	go func() { c.TestInvalidVariableNames(); wg.Done() }()
	wg.Wait()
}
