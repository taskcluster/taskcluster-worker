package mockengine

import (
	"encoding/json"
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

func TestAttachEngine(t *testing.T) {
	// TODO: This is supposed to be arguments to:
	// var testcase = enginetest.AttachCacheTestCase{
	//   engineName: ...,
	//   mountpoint: ...,
	//   ...
	// }
	//
	// So we can do things like:
	//
	// func TestAssertVolumePayload(t *testing.T) {
	//   testcase.TestAssertVolumePayload(t)
	// }
	// func TestAttachVolume(t *testing.T) {
	//   testcase.TestAttachVolume(t)
	// }
	//
	// This way once we've declared a few simple things we have a test case object
	// and we can write a bunch of generic tests and does all sort of stupid
	// simple tests using the test payloads given.
	engineName := "mock"
	mountpoint := "/mock/volume"
	writeVolumePayload := `{
    "start": {
      "delay": 10,
      "function": "set-volume",
      "argument": "/mock/volume"
    }
  }`
	assertVolumePayload := `{
    "start": {
      "delay": 10,
      "function": "get-volume",
      "argument": "/mock/volume"
    }
  }`

	t.Parallel()

	// Find EngineProvider
	engineProvider := extpoints.EngineProviders.Lookup(engineName)
	if engineProvider == nil {
		t.Fatal("Couldn't find EngineProvider: ", engineName)
	}

	// Create Engine instance
	engine, err := engineProvider(extpoints.EngineOptions{
		Environment: nil, //TODO: Provide something we can use for tests
	})
	if err != nil {
		t.Fatal("Failed to create Engine: ", err)
	}

	// Construct SandboxBuilder
	sandboxBuilder, err := engine.NewSandboxBuilder(engines.SandboxOptions{
		TaskContext: nil, // TODO: Create a TaskContext
		Payload:     parseTestPayload(t, engine, writeVolumePayload),
	})
	if err != nil {
		t.Fatal("Error creating SandboxBuilder: ", err)
	}

	// Make a volume
	volume, err := engine.NewCacheFolder()
	if err != nil {
		t.Fatal("Failed to create a new cache folder: ", err)
	}

	// Attach volume to sandbox
	err = sandboxBuilder.AttachVolume(mountpoint, volume, false)
	if err != nil {
		t.Fatal("Failed to attach volume: ", err)
	}

	// Start sandbox and wait for result
	sandbox, err := sandboxBuilder.StartSandbox()
	if err != nil {
		t.Fatal("Failed to start sandbox: ", err)
	}
	// Wait for result
	resultSet, err := sandbox.WaitForResult()
	if err != nil {
		t.Fatal("WaitForResult failed: ", err)
	}

	// Check for success
	if !resultSet.Success() {
		t.Error("Running with writeVolumePayload didn't finish successfully")
	}
	// Dispose resultSet
	err = resultSet.Dispose()
	if err != nil {
		t.Error("Failed to dispose of ResultSet: ", err)
	}

	// Construct SandboxBuilder to assert that something was written to the
	// volume, Basically the same as before, but different payload and same
	// volume instance.
	sb2, err := engine.NewSandboxBuilder(engines.SandboxOptions{
		TaskContext: nil, // TODO: Create a TaskContext
		Payload:     parseTestPayload(t, engine, assertVolumePayload),
	})
	if err != nil {
		t.Fatal("Error creating SandboxBuilder: ", err)
	}

	// Attach volume to sandbox
	err = sb2.AttachVolume(mountpoint, volume, false)
	if err != nil {
		t.Fatal("Failed to attach volume: ", err)
	}

	// Start sandbox and wait for result
	s, err := sb2.StartSandbox()
	if err != nil {
		t.Fatal("Failed to start sandbox: ", err)
	}
	// Wait for result
	rs, err := s.WaitForResult()
	if err != nil {
		t.Fatal("WaitForResult failed: ", err)
	}

	// Check for success
	if !rs.Success() {
		t.Error("Running with assertVolumePayload didn't finish successfully")
	}
	// Dispose resultSet
	err = rs.Dispose()
	if err != nil {
		t.Error("Failed to dispose of ResultSet: ", err)
	}
}
