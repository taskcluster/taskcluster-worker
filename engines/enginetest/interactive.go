package enginetest

import "github.com/taskcluster/taskcluster-worker/engines"

// The ShellTestCase contains information sufficient to test the interactive
// shell provided by a Sandbox
type ShellTestCase struct {
	Engine string
	// Command to pipe to the Shell over stdin
	Command string
	// Result to expect from the Shell on stdout
	Stdout string
	// Result to expect from the Shell on stderr
	Stderr string
	// Payload for the engine that will contain an interactive environment as
	// described above.
	Payload string
}

// The DisplayTestCase contains information sufficient to test the interactive
// display provided by a Sandbox
type DisplayTestCase struct {
	Engine string
	// List of display that should be returned from Sandbox.ListDisplays(),
	// They will all be opened to ensure that they are in fact VNC connections.
	Displays []engines.Display
	// Name of a display that does not exist, it will be attempted opened to
	// check that this failure is handled gracefully.
	InvalidDisplayName string
	// Payload for the engine that will contain an interactive environment as
	// described above.
	Payload string
}
