package runtime

import (
	"fmt"
	"io"
)

// The SandboxContextBuilder structure contains methods
// that plugins can use to build the SandboxContext
//
// Note, all method on this object must be thread-safe,
// as they are intended to be called by different
// plugins in the newPlugin() method.
type SandboxContextBuilder struct {
	logDrains []io.Writer
}

// AttachLogDrain takes an io.Writer drain for logs to
// be written to.
//
// This allows multiple plugins to consume log messages,
// whether they want stream, upload or aggregate the
// anything written to SandboxContext.LogDrain() will
// also be written to the drain given here.
func (b *SandboxContextBuilder) AttachLogDrain(drain io.Writer) {
	b.logDrains = append(b.logDrains, drain)
}

// newSandboxContext creates a new SandboxContext
func (b *SandboxContextBuilder) newSandboxContext() *SandboxContext {
	return &SandboxContext{
		logDrain: io.MultiWriter(b.logDrains...),
	}
}

// The SandboxContext structure contains auxiliary methods
type SandboxContext struct {
	logDrain io.Writer
}

// LogDrain returns an io.Writer that raw Sandbox logs should be written to.
func (c *SandboxContext) LogDrain() io.Writer {
	return c.logDrain
}

// Log writes a log message from the sandbox, these log messages will be
// prefixed "[taskcluster]" so it's easy to see to that they are worker logs.
func (c *SandboxContext) Log(a ...interface{}) {
	a = append([]interface{}{"[taskcluster] "}, a...)
	_, err := fmt.Fprintln(c.logDrain, a...)
	if err != nil {
		panic(err)
	}
}
