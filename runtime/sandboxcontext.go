package runtime

import (
	"fmt"
	"io"
)

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
