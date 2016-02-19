package enginetest

import (
	"encoding/json"
	"fmt"
	"os"
	rt "runtime"
	"sync"

	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/extpoints"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/gc"
)

func fmtPanic(a ...interface{}) {
	panic(fmt.Sprintln(a...))
}

func nilOrpanic(err error, a ...interface{}) {
	if err != nil {
		fmtPanic(append(a, err)...)
	}
}

func evalNilOrPanic(f func() error, a ...interface{}) {
	nilOrpanic(f(), a...)
}

// Type can embed so that we can reuse ensure engine
type engineProvider struct {
	sync.Mutex
	engine      engines.Engine
	environment *runtime.Environment
}

func (p *engineProvider) ensureEngine(engineName string) {
	p.Lock()
	defer p.Unlock()
	if p.engine != nil {
		return
	}
	// Create a runtime environment
	p.environment = newTestEnvironment()
	// Find EngineProvider
	engineProvider := extpoints.EngineProviders.Lookup(engineName)
	if engineProvider == nil {
		fmtPanic("Couldn't find EngineProvider: ", engineName)
	}
	// Create Engine instance
	engine, err := engineProvider.NewEngine(extpoints.EngineOptions{
		Environment: p.environment,
		Log:         p.environment.Log.WithField("engine", engineName),
	})
	nilOrpanic(err, "Failed to create Engine")
	p.engine = engine
}

func (p *engineProvider) newTestTaskContext() (*runtime.TaskContext, *runtime.TaskContextController) {
	ctx, control, err := runtime.NewTaskContext(p.environment.TemporaryStorage.NewFilePath())
	nilOrpanic(err, "Failed to create new TaskContext")
	return ctx, control
}

// NewTestEnvironment creates a new Environment suitable for use in tests.
//
// This function should only be used in testing
func newTestEnvironment() *runtime.Environment {
	storage, err := runtime.NewTemporaryStorage(os.TempDir())
	nilOrpanic(err, "Failed to create temporary storage at: ", os.TempDir())

	folder, err := storage.NewFolder()
	nilOrpanic(err, "Failed to create temporary storage folder")

	// Set finalizer so that we always get the temporary folder removed.
	// This is should really only be used in tests, otherwise it would better to
	// call Remove() manually.
	rt.SetFinalizer(folder, func(f runtime.TemporaryFolder) {
		f.Remove()
	})

	logger, err := runtime.CreateLogger(os.Getenv("LOGGING_LEVEL"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating logger. %s", err)
		os.Exit(1)
	}

	return &runtime.Environment{
		GarbageCollector: &gc.GarbageCollector{},
		TemporaryStorage: folder,
		Log:              logger,
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
	defer evalNilOrPanic(resultSet.Dispose, "Failed to dispose of ResultSet")
	return resultSet.Success()
}
