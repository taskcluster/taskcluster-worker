package enginetest

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/extpoints"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

// Type can embed so that we can reuse ensure engine
type engineProvider struct {
	sync.Mutex
	engine engines.Engine
}

func (p *engineProvider) ensureEngine(engineName string) {
	p.Lock()
	defer p.Unlock()
	if p.engine != nil {
		return
	}
	// Find EngineProvider
	engineProvider := extpoints.EngineProviders.Lookup(engineName)
	if engineProvider == nil {
		fmtPanic("Couldn't find EngineProvider: ", engineName)
	}
	// Create Engine instance
	engine, err := engineProvider(extpoints.EngineOptions{
		Environment: runtime.NewTestEnvironment(),
	})
	nilOrpanic(err, "Failed to create Engine")
	p.engine = engine
}

func fmtPanic(a ...interface{}) {
	panic(fmt.Sprintln(a...))
}

func nilOrpanic(err error, a ...interface{}) {
	if err != nil {
		fmtPanic(append(a, err)...)
	}
}

func parseTestPayload(engine engines.Engine, payload string) interface{} {
	jsonPayload := map[string]json.RawMessage{}
	err := json.Unmarshal([]byte(payload), &jsonPayload)
	nilOrpanic(err, "Payload parsing failed: ", payload)
	p, err := engine.PayloadSchema().Parse(jsonPayload)
	nilOrpanic(err, "Payload validation failed: ", payload)
	return p
}

func buildRunSandbox(b engines.SandboxBuilder) bool {
	// Start sandbox and wait for result
	sandbox, err := b.StartSandbox()
	nilOrpanic(err, "Failed to start sandbox")

	// Wait for result
	resultSet, err := sandbox.WaitForResult()
	nilOrpanic(err, "WaitForResult failed")

	// Get result and dispose ResultSet
	result := resultSet.Success()
	nilOrpanic(resultSet.Dispose(), "Failed to dispose of ResultSet: ")
	return result
}
