package enginetest

import (
	"fmt"
	"io"
	"io/ioutil"
	"sync"
	"time"

	vnc "github.com/mitchellh/go-vnc"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

// The DisplayTestCase contains information sufficient to test the interactive
// display provided by a Sandbox
type DisplayTestCase struct {
	*EngineProvider
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

// TestListDisplays tests that listDisplays works and returns Displays
func (c *DisplayTestCase) TestListDisplays() {
	debug("## TestListDisplays")
	r := c.newRun()
	defer r.Dispose()
	r.NewSandboxBuilder(c.Payload)
	r.StartSandbox()

	// List displays
	displays, err := r.sandbox.ListDisplays()
	nilOrPanic(err, "Failed to ListDisplays")

	// Check that we has many displays as we declared
	assert(len(displays) == len(c.Displays), "Expected: ", len(c.Displays),
		" displays, but we only got: ", len(displays))

	// Check that displays are all declared
	for _, display := range displays {
		ok := false
		for _, d := range c.Displays {
			if d.Name == display.Name {
				ok = true
				// Test properties that are declared to not have empty value
				assert(d.Description == "" || d.Description == display.Description,
					"Description was declared and didn't match, got: ", display.Description)
				assert(d.Width == 0 || d.Width == display.Width,
					"Width was declared and didn't match, got: ", display.Width)
				assert(d.Height == 0 || d.Height == display.Height,
					"Height was declared and didn't match, got: ", display.Height)
			}
		}
		if !ok {
			panic(fmt.Sprintf("ListDisplays returned unexpected display: %#v", display))
		}
	}
}

// TestDisplays tests that we can connect to all Displays listed, and that the
// resolution is correct if advertized (ie. non-zero). To facilitate that
// resolution changes in the test sandbox this test will only require that the
// resolution either before or after connecting matches what is listed.
func (c *DisplayTestCase) TestDisplays() {
	debug("## TestDisplays")
	r := c.newRun()
	defer r.Dispose()
	r.NewSandboxBuilder(c.Payload)
	r.StartSandbox()

	// List displays
	displays, err := r.sandbox.ListDisplays()
	nilOrPanic(err, "Failed to ListDisplays")
	assert(len(displays) > 0, "Expected at least one display")

	// Test each display
	resToCheckLater := make(map[string]resolution)
	for _, display := range displays {
		debug(" - Opening display: %s", display.Name)
		c, err2 := r.sandbox.OpenDisplay(display.Name)
		nilOrPanic(err2, "Failed to OpenDisplay for: ", display.Name)

		debug(" - Querying for resolution")
		res, err2 := getDisplayResolution(c)
		nilOrPanic(err2, "Failed to connect to display, err: ")
		debug(" - Got resolution, width: %d, height: %d", res.width, res.height)

		// If the actual resolution doesn't match what ListDisplays returned we
		// will check it later, running ListDisplays again to ensure we support
		// resolutions changing at least once during testing
		if res.width != display.Width || res.height != display.Height {
			resToCheckLater[display.Name] = res
		}
	}

	displays, err = r.sandbox.ListDisplays()
	nilOrPanic(err, "Failed to ListDisplays 2nd time")
	for _, display := range displays {
		// Don't check any that haven't got a resolution
		if display.Width == 0 && display.Height == 0 {
			continue
		}
		// Only check, the ones that we recorded we needed to check later
		if res, ok := resToCheckLater[display.Name]; ok {
			assert(res.width == display.Width && res.height == display.Height,
				"Resolution was defined in ListDisplays, but didn't match VNC client")
		}
	}
}

// TestKillDisplay opens a display and test that it is closed if Sandbox.Kill()
// is called.
func (c *DisplayTestCase) TestKillDisplay() {
	debug("## TestKillDisplay")
	r := c.newRun()
	defer r.Dispose()
	r.NewSandboxBuilder(c.Payload)
	r.StartSandbox()

	// List displays
	displays, err := r.sandbox.ListDisplays()
	nilOrPanic(err, "Failed to ListDisplays")
	assert(len(displays) > 0, "Expected at least one display")

	display := displays[0]
	debug(" - Opening display: %s", display.Name)
	conn, err := r.sandbox.OpenDisplay(display.Name)
	nilOrPanic(err, "Failed to OpenDisplay for: ", display.Name)

	// get signal when conn is closed
	closed := atomics.Barrier{}
	go func() {
		defer closed.Fall()
		io.Copy(ioutil.Discard, conn)
	}()

	// Sleep 100ms
	time.Sleep(100 * time.Millisecond)
	assert(!closed.IsFallen(), "display connection closed too soon")

	// kill sandbox
	err = r.sandbox.Kill()
	nilOrPanic(err, "failed to kill sandbox")
	// hoping 100ms is enough to read out the end-of-stream
	time.Sleep(100 * time.Millisecond)
	assert(closed.IsFallen(), "expected display connection to be closed")
}

// TestInvalidDisplayName test that IpenDisplay on InvalidDisplayName is
// properly handled.
func (c *DisplayTestCase) TestInvalidDisplayName() {
	debug("## TestInvalidDisplayName")
	r := c.newRun()
	defer r.Dispose()
	r.NewSandboxBuilder(c.Payload)
	r.StartSandbox()

	conn, err := r.sandbox.OpenDisplay(c.InvalidDisplayName)
	assert(err == engines.ErrNoSuchDisplay, "Expected ErrNoSuchDisplay")
	assert(conn == nil, "Expected nil when we got error")
}

// Test runs all tests in parallel
func (c *DisplayTestCase) Test() {
	wg := sync.WaitGroup{}
	wg.Add(4)
	go func() { c.TestListDisplays(); wg.Done() }()
	go func() { c.TestDisplays(); wg.Done() }()
	go func() { c.TestInvalidDisplayName(); wg.Done() }()
	go func() { c.TestKillDisplay(); wg.Done() }()
	wg.Wait()
}

type resolution struct {
	width  int
	height int
}

func getDisplayResolution(c io.ReadWriteCloser) (resolution, error) {
	client, err := vnc.Client(ioext.NopConn(c), &vnc.ClientConfig{})
	if err != nil {
		return resolution{}, err
	}
	client.Close()
	return resolution{
		width:  int(client.FrameBufferWidth),
		height: int(client.FrameBufferHeight),
	}, nil
}
