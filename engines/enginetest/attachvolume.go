// Package enginetest provides utilities for testing generic engine
// implementations.
package enginetest

import (
	"log"

	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime/mocks"
)

// A VolumeTestCase holds information necessary to run tests that an engine
// can create volumes, mount and read/write to volumes.
type VolumeTestCase struct {
	*EngineProvider
	// A valid mountpoint
	Mountpoint string
	// A task.payload as accepted by the engine, which will write something to the
	// mountpoint given.
	WriteVolumePayload string
	// A task.payload as accepted by the engine, which will check that something
	// was written to the mountpoint given.
	CheckVolumePayload string
}

// Construct SandboxBuilder, Attach volume to sandbox and run it
func (c *VolumeTestCase) writeVolume(volume engines.Volume, readOnly bool) bool {
	ctx, control := c.newTestTaskContext([]string{})
	defer evalNilOrPanic(control.Dispose, "Failed to dispose TaskContext")
	defer evalNilOrPanic(control.CloseLog, "Failed to close log")

	sandboxBuilder, err := c.engine.NewSandboxBuilder(engines.SandboxOptions{
		TaskContext: ctx,
		Payload:     parseTestPayload(c.engine, c.WriteVolumePayload),
		Monitor:     mocks.NewMockMonitor(true),
	})
	nilOrPanic(err, "Error creating SandboxBuilder")
	err = sandboxBuilder.AttachVolume(c.Mountpoint, volume, readOnly)
	nilOrPanic(err, "Failed to attach volume")
	return buildRunSandbox(sandboxBuilder)
}

// Construct SandboxBuilder, Attach volume to sandbox and run it
func (c *VolumeTestCase) readVolume(volume engines.Volume, readOnly bool) bool {
	r := c.newRun()
	defer r.Dispose()
	r.NewSandboxBuilder(c.CheckVolumePayload)
	err := r.sandboxBuilder.AttachVolume(c.Mountpoint, volume, readOnly)
	nilOrPanic(err, "Failed to attach volume")
	return r.buildRunSandbox()
}

// TestWriteReadVolume tests that we can write and read from a volume
func (c *VolumeTestCase) TestWriteReadVolume() {
	c.ensureEngine()
	volume, err := c.engine.NewVolume(map[string]interface{}{})
	nilOrPanic(err, "Failed to create a new cache folder")
	defer evalNilOrPanic(volume.Dispose, "Failed to dispose cache folder")
	if !c.writeVolume(volume, false) {
		log.Panic("Running with writeVolumePayload didn't finish successfully")
	}
	if !c.readVolume(volume, false) {
		log.Panic(
			"Running with CheckVolumePayload didn't finish successfully,",
			"after we ran writeVolumePayload with same volume (writing something)",
		)
	}
}

// TestReadEmptyVolume tests that read from empty volume doesn't work
func (c *VolumeTestCase) TestReadEmptyVolume() {
	c.ensureEngine()
	volume, err := c.engine.NewVolume(map[string]interface{}{})
	nilOrPanic(err, "Failed to create a new cache folder")
	defer evalNilOrPanic(volume.Dispose, "Failed to dispose cache folder")
	if c.readVolume(volume, false) {
		log.Panic(
			"Running with CheckVolumePayload with an empty volume was successful.",
			"It really shouldn't have been.",
		)
	}
}

// TestWriteToReadOnlyVolume tests that write doesn't work to a read-only volume
func (c *VolumeTestCase) TestWriteToReadOnlyVolume() {
	c.ensureEngine()
	volume, err := c.engine.NewVolume(map[string]interface{}{})
	nilOrPanic(err, "Failed to create a new cache folder")
	defer evalNilOrPanic(volume.Dispose, "Failed to dispose cache folder")
	c.writeVolume(volume, true)
	if c.readVolume(volume, false) {
		log.Panic("Write on read-only volume didn't give us is an issue when reading")
	}
}

// TestReadToReadOnlyVolume tests that we can read from a read-only volume
func (c *VolumeTestCase) TestReadToReadOnlyVolume() {
	c.ensureEngine()
	volume, err := c.engine.NewVolume(map[string]interface{}{})
	nilOrPanic(err, "Failed to create a new cache folder")
	defer evalNilOrPanic(volume.Dispose, "Failed to dispose cache folder")
	if !c.writeVolume(volume, false) {
		log.Panic("Running with writeVolumePayload didn't finish successfully")
	}
	if !c.readVolume(volume, true) {
		log.Panic(
			"Running with CheckVolumePayload didn't finish successfully, ",
			"after we ran writeVolumePayload with same volume (writing something) ",
			"This was with a readOnly attachment when reading",
		)
	}
}

// Test runs all tests on the test case.
func (c *VolumeTestCase) Test() {
	c.TestWriteReadVolume()
	c.TestReadEmptyVolume()
	c.TestWriteToReadOnlyVolume()
	c.TestReadToReadOnlyVolume()
}
