package runtime

import (
	"fmt"
	"io"
	"sync"
)

// The SandboxContextBuilder structure contains methods
// that plugins can use to build the SandboxContext
//
// Note, all method on this object must be thread-safe,
// as they are intended to be called by different
// plugins in the newPlugin() method.
type SandboxContextBuilder struct {
	m         sync.Mutex
	logDrains []io.Writer
	disposed  bool
}

// AttachLogDrain takes an io.Writer drain for logs to
// be written to.
//
// This allows multiple plugins to consume log messages,
// whether they want stream, upload or aggregate the
// anything written to SandboxContext.LogDrain() will
// also be written to the drain given here.
func (b *SandboxContextBuilder) AttachLogDrain(drain io.Writer) {
	// Get the lock
	b.m.Lock()
	defer b.m.Unlock()
	// Check that newSandboxContext() haven't been called
	// yet. Plugins may not keep references to this object
	// as using it after NewPlugin() isn't supported.
	if b.disposed {
		panic("SandboxContextBuilder cannot be used after NewPlugin()!")
	}
	// Add log drain to list
	b.logDrains = append(b.logDrains, drain)
}

// newSandboxContext creates a new SandboxContext
func (b *SandboxContextBuilder) newSandboxContext() *SandboxContext {
	// Get the lock
	b.m.Lock()
	defer b.m.Unlock()
	// Ensure that newSandboxContext() is only called once
	if b.disposed {
		panic("newSandboxContext() can only be called once")
	}
	b.disposed = true
	// Create SandboxContext
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
