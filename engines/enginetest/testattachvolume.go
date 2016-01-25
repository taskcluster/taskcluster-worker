// Package enginetest provides utilities for testing generic engine
// implementations.
package enginetest

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/extpoints"
)

func parseTestPayload(t *testing.T, engine engines.Engine, payload string) interface{} {
	jsonPayload := map[string]json.RawMessage{}
	err := json.Unmarshal([]byte(payload), &jsonPayload)
	if err != nil {
		t.Fatal("Test payload parsing failed: ", err, " payload: ", payload)
	}
	p, err := engine.PayloadSchema().Parse(jsonPayload)
	if err != nil {
		t.Fatal("Test payload validation failed: ", err, " payload: ", payload)
	}
	return p
}

// A VolumeTestCase holds information necessary to run tests that an engine
// can create volumes, mount and read/write to volumes.
type VolumeTestCase struct {
	sync.Mutex
	Engine             string
	Mountpoint         string
	WriteVolumePayload string
	CheckVolumePayload string
	engine             engines.Engine
}

func nilOrFatal(t *testing.T, err error, a ...interface{}) {
	if err != nil {
		t.Fatal(append(a, err)...)
	}
}

func nilOrError(t *testing.T, err error, a ...interface{}) {
	if err != nil {
		t.Error(append(a, err)...)
	}
}

func (c *VolumeTestCase) ensureEngine(t *testing.T) {
	c.Lock()
	defer c.Unlock()
	if c.engine != nil {
		return
	}
	// Find EngineProvider
	engineProvider := extpoints.EngineProviders.Lookup(c.Engine)
	if engineProvider == nil {
		t.Fatal("Couldn't find EngineProvider: ", c.Engine)
	}
	// Create Engine instance
	engine, err := engineProvider(extpoints.EngineOptions{
		Environment: nil, //TODO: Provide something we can use for tests
	})
	nilOrFatal(t, err, "Failed to create Engine")
	c.engine = engine
}

func (c *VolumeTestCase) buildRunSandbox(t *testing.T, b engines.SandboxBuilder) bool {
	// Start sandbox and wait for result
	sandbox, err := b.StartSandbox()
	nilOrFatal(t, err, "Failed to start sandbox")

	// Wait for result
	resultSet, err := sandbox.WaitForResult()
	nilOrFatal(t, err, "WaitForResult failed")

	// Get result and dispose ResultSet
	result := resultSet.Success()
	nilOrError(t, resultSet.Dispose(), "Failed to dispose of ResultSet: ")
	return result
}

func (c *VolumeTestCase) writeVolume(t *testing.T, volume engines.Volume, readOnly bool) bool {
	// Construct SandboxBuilder, Attach volume to sandbox and run it
	sandboxBuilder, err := c.engine.NewSandboxBuilder(engines.SandboxOptions{
		TaskContext: nil, // TODO: Create a TaskContext
		Payload:     parseTestPayload(t, c.engine, c.WriteVolumePayload),
	})
	nilOrFatal(t, err, "Error creating SandboxBuilder")
	err = sandboxBuilder.AttachVolume(c.Mountpoint, volume, readOnly)
	nilOrFatal(t, err, "Failed to attach volume")
	return c.buildRunSandbox(t, sandboxBuilder)
}

func (c *VolumeTestCase) readVolume(t *testing.T, volume engines.Volume, readOnly bool) bool {
	// Construct SandboxBuilder, Attach volume to sandbox and run it
	sandboxBuilder, err := c.engine.NewSandboxBuilder(engines.SandboxOptions{
		TaskContext: nil, // TODO: Create a TaskContext
		Payload:     parseTestPayload(t, c.engine, c.CheckVolumePayload),
	})
	nilOrFatal(t, err, "Error creating SandboxBuilder")
	err = sandboxBuilder.AttachVolume(c.Mountpoint, volume, readOnly)
	nilOrFatal(t, err, "Failed to attach volume")
	return c.buildRunSandbox(t, sandboxBuilder)
}

// TestWriteReadVolume tests that we can write and read from a volume
func (c *VolumeTestCase) TestWriteReadVolume(t *testing.T) {
	c.ensureEngine(t)
	volume, err := c.engine.NewCacheFolder()
	nilOrFatal(t, err, "Failed to create a new cache folder")
	if !c.writeVolume(t, volume, false) {
		t.Fatal("Running with writeVolumePayload didn't finish successfully")
	}
	if !c.readVolume(t, volume, false) {
		t.Fatal("Running with CheckVolumePayload didn't finish successfully, ",
			"after we ran writeVolumePayload with same volume (writing something)")
	}
	nilOrFatal(t, volume.Dispose(), "Failed to dispose cache folder")
}

// TestReadEmptyVolume tests that read from empty volume doesn't work
func (c *VolumeTestCase) TestReadEmptyVolume(t *testing.T) {
	c.ensureEngine(t)
	volume, err := c.engine.NewCacheFolder()
	nilOrFatal(t, err, "Failed to create a new cache folder")
	if c.readVolume(t, volume, false) {
		t.Fatal("Running with CheckVolumePayload with an empty volume was successful.",
			"It really shouldn't have been.")
	}
	nilOrFatal(t, volume.Dispose(), "Failed to dispose new cache folder 2")
}

// TestWriteToReadOnlyVolume tests that write doesn't work to a read-only volume
func (c *VolumeTestCase) TestWriteToReadOnlyVolume(t *testing.T) {
	c.ensureEngine(t)
	volume, err := c.engine.NewCacheFolder()
	nilOrFatal(t, err, "Failed to create a new cache folder")
	c.writeVolume(t, volume, true)
	if c.readVolume(t, volume, false) {
		t.Fatal("Write on read-only volume didn't give us is an issue when reading")
	}
	nilOrFatal(t, volume.Dispose(), "Failed to dispose cache folder")
}

// TestReadToReadOnlyVolume tests that we can read from a read-only volume
func (c *VolumeTestCase) TestReadToReadOnlyVolume(t *testing.T) {
	c.ensureEngine(t)
	volume, err := c.engine.NewCacheFolder()
	nilOrFatal(t, err, "Failed to create a new cache folder")
	if !c.writeVolume(t, volume, false) {
		t.Fatal("Running with writeVolumePayload didn't finish successfully")
	}
	if !c.readVolume(t, volume, true) {
		t.Fatal("Running with CheckVolumePayload didn't finish successfully, ",
			"after we ran writeVolumePayload with same volume (writing something) ",
			"This was with a readOnly attachment when reading")
	}
	nilOrFatal(t, volume.Dispose(), "Failed to dispose cache folder")
}

// Test runs all tests on the test case.
func (c *VolumeTestCase) Test(t *testing.T) {
	c.ensureEngine(t)
	wg := sync.WaitGroup{}
	wg.Add(4)
	go func() { c.TestWriteReadVolume(t); wg.Done() }()
	go func() { c.TestReadEmptyVolume(t); wg.Done() }()
	go func() { c.TestWriteToReadOnlyVolume(t); wg.Done() }()
	go func() { c.TestReadToReadOnlyVolume(t); wg.Done() }()
	wg.Wait()
}
