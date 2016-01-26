// Package enginetest provides utilities for testing generic engine
// implementations.
package enginetest

import (
	"sync"
	"testing"

	"github.com/taskcluster/taskcluster-worker/engines"
)

// A VolumeTestCase holds information necessary to run tests that an engine
// can create volumes, mount and read/write to volumes.
type VolumeTestCase struct {
	engineProvider
	Engine             string
	Mountpoint         string
	WriteVolumePayload string
	CheckVolumePayload string
}

func (c *VolumeTestCase) writeVolume(volume engines.Volume, readOnly bool) bool {
	// Construct SandboxBuilder, Attach volume to sandbox and run it
	sandboxBuilder, err := c.engine.NewSandboxBuilder(engines.SandboxOptions{
		TaskContext: nil, // TODO: Create a TaskContext
		Payload:     parseTestPayload(c.engine, c.WriteVolumePayload),
	})
	nilOrpanic(err, "Error creating SandboxBuilder")
	err = sandboxBuilder.AttachVolume(c.Mountpoint, volume, readOnly)
	nilOrpanic(err, "Failed to attach volume")
	return buildRunSandbox(sandboxBuilder)
}

func (c *VolumeTestCase) readVolume(volume engines.Volume, readOnly bool) bool {
	// Construct SandboxBuilder, Attach volume to sandbox and run it
	sandboxBuilder, err := c.engine.NewSandboxBuilder(engines.SandboxOptions{
		TaskContext: nil, // TODO: Create a TaskContext
		Payload:     parseTestPayload(c.engine, c.CheckVolumePayload),
	})
	nilOrpanic(err, "Error creating SandboxBuilder")
	err = sandboxBuilder.AttachVolume(c.Mountpoint, volume, readOnly)
	nilOrpanic(err, "Failed to attach volume")
	return buildRunSandbox(sandboxBuilder)
}

// TestWriteReadVolume tests that we can write and read from a volume
func (c *VolumeTestCase) TestWriteReadVolume() {
	c.ensureEngine(c.Engine)
	volume, err := c.engine.NewCacheFolder()
	nilOrpanic(err, "Failed to create a new cache folder")
	if !c.writeVolume(volume, false) {
		fmtPanic("Running with writeVolumePayload didn't finish successfully")
	}
	if !c.readVolume(volume, false) {
		fmtPanic("Running with CheckVolumePayload didn't finish successfully, ",
			"after we ran writeVolumePayload with same volume (writing something)")
	}
	nilOrpanic(volume.Dispose(), "Failed to dispose cache folder")
}

// TestReadEmptyVolume tests that read from empty volume doesn't work
func (c *VolumeTestCase) TestReadEmptyVolume() {
	c.ensureEngine(c.Engine)
	volume, err := c.engine.NewCacheFolder()
	nilOrpanic(err, "Failed to create a new cache folder")
	if c.readVolume(volume, false) {
		fmtPanic("Running with CheckVolumePayload with an empty volume was successful.",
			"It really shouldn't have been.")
	}
	nilOrpanic(volume.Dispose(), "Failed to dispose new cache folder 2")
}

// TestWriteToReadOnlyVolume tests that write doesn't work to a read-only volume
func (c *VolumeTestCase) TestWriteToReadOnlyVolume() {
	c.ensureEngine(c.Engine)
	volume, err := c.engine.NewCacheFolder()
	nilOrpanic(err, "Failed to create a new cache folder")
	c.writeVolume(volume, true)
	if c.readVolume(volume, false) {
		fmtPanic("Write on read-only volume didn't give us is an issue when reading")
	}
	nilOrpanic(volume.Dispose(), "Failed to dispose cache folder")
}

// TestReadToReadOnlyVolume tests that we can read from a read-only volume
func (c *VolumeTestCase) TestReadToReadOnlyVolume() {
	c.ensureEngine(c.Engine)
	volume, err := c.engine.NewCacheFolder()
	nilOrpanic(err, "Failed to create a new cache folder")
	if !c.writeVolume(volume, false) {
		fmtPanic("Running with writeVolumePayload didn't finish successfully")
	}
	if !c.readVolume(volume, true) {
		fmtPanic("Running with CheckVolumePayload didn't finish successfully, ",
			"after we ran writeVolumePayload with same volume (writing something) ",
			"This was with a readOnly attachment when reading")
	}
	nilOrpanic(volume.Dispose(), "Failed to dispose cache folder")
}

// Test runs all tests on the test case.
func (c *VolumeTestCase) Test(t *testing.T) {
	c.ensureEngine(c.Engine)
	wg := sync.WaitGroup{}
	wg.Add(4)
	go func() { c.TestWriteReadVolume(); wg.Done() }()
	go func() { c.TestReadEmptyVolume(); wg.Done() }()
	go func() { c.TestWriteToReadOnlyVolume(); wg.Done() }()
	go func() { c.TestReadToReadOnlyVolume(); wg.Done() }()
	wg.Wait()
}
