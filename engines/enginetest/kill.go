package enginetest

import (
	"bytes"
	"strings"
)

// A KillTestCase tests if calling Sandbox.Kill() works by invoking it after
// Target has been printed to log by Payload.
type KillTestCase struct {
	*EngineProvider
	Target  string
	Payload string
}

// Test runs the test case
func (c KillTestCase) Test() {
	debug(" - New run")
	r := c.newRun()
	defer r.Dispose()
	debug(" - New sandbox builder")
	r.NewSandboxBuilder(c.Payload)

	debug(" - Start sandbox")
	r.StartSandbox()

	go func() {
		r.OpenLogReader()
		buf := bytes.Buffer{}
		for !strings.Contains(buf.String(), c.Target) {
			b := []byte{0}
			n, err := r.logReader.Read(b)
			if n != 1 {
				panic("Expected one byte to be read!")
			}
			buf.WriteByte(b[0])
			nilOrPanic(err, "Failed while reading from livelog...")
		}
		nilOrPanic(r.sandbox.Kill(), "Sandbox.Kill() returned an error")
	}()

	debug(" - Wait for result")
	success := r.WaitForResult()
	debug(" - Result: %v", success)
	assert(!success, "Expected Kill() to result in false")
}
