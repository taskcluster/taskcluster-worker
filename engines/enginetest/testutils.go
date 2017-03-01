package enginetest

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	rt "runtime"
	"strings"
	"sync"

	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/gc"
	"github.com/taskcluster/taskcluster-worker/runtime/mocks"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

var debug = util.Debug("enginetest")

func nilOrPanic(err error, a ...interface{}) {
	if err != nil {
		log.Panic(append(a, err)...)
	}
}

func evalNilOrPanic(f func() error, a ...interface{}) {
	nilOrPanic(f(), a...)
}

func assert(condition bool, a ...interface{}) {
	if !condition {
		log.Panic(a...)
	}
}

// EngineProvider is a base object used by test case to get an engine.
type EngineProvider struct {
	m           sync.Mutex
	engine      engines.Engine
	refCount    int
	environment *runtime.Environment
	// Name of engine
	Engine string
	// Engine configuration as JSON
	Config string
	// Function to be called before using the engine, may return a function to be
	// called after running the engine.
	Setup func() func()
}

// SetupEngine will initialize the engine and hold reference to it.
// This allows the same engine instance to be reused between tests.
// TearDownEngine must be called later to ensure the engine is destroyed.
//
// This is not necessary, but can be used to improve test efficiency and
// reliability in cases where engine setup/teardown is flaky/slow.
func (p *EngineProvider) SetupEngine() {
	p.ensureEngine()
}

// TearDownEngine the opposite of SetupEngine.
func (p *EngineProvider) TearDownEngine() {
	p.releaseEngine()
}

func (p *EngineProvider) ensureEngine() {
	p.m.Lock()
	defer p.m.Unlock()
	p.refCount++
	if p.engine != nil {
		return
	}
	// Create a runtime environment
	p.environment = newTestEnvironment()
	// Find EngineProvider
	engineProvider := engines.Engines()[p.Engine]
	if engineProvider == nil {
		panic(fmt.Sprint("Couldn't find EngineProvider: ", p.Engine))
	}

	var jsonConfig interface{}
	err := json.Unmarshal([]byte(p.Config), &jsonConfig)
	nilOrPanic(err, "Config parsing failed: ", p.Config)
	err = engineProvider.ConfigSchema().Validate(jsonConfig)
	nilOrPanic(err, "Config validation failed: ", p.Config, "\nError: ", err)

	// Create Engine instance
	engine, err := engineProvider.NewEngine(engines.EngineOptions{
		Environment: p.environment,
		Monitor:     p.environment.Monitor.WithTag("engine", p.Engine),
		Config:      jsonConfig,
	})
	nilOrPanic(err, "Failed to create Engine")
	p.engine = engine
}

func (p *EngineProvider) releaseEngine() {
	p.m.Lock()
	defer p.m.Unlock()

	if p.engine == nil {
		log.Panic("releaseEngine() but we don't have an active engine")
	}
	if p.refCount <= 0 {
		log.Panic("releaseEngine() but refCount <= 0")
	}
	p.refCount--
	if p.refCount <= 0 {
		err := p.engine.Dispose()
		nilOrPanic(err, "engine.Dispose(), error: ")
		p.refCount = 0
		p.engine = nil
	}
}

func (p *EngineProvider) newTestTaskContext() (*runtime.TaskContext, *runtime.TaskContextController) {
	ctx, control, err := runtime.NewTaskContext(p.environment.TemporaryStorage.NewFilePath(), runtime.TaskInfo{}, nil)
	nilOrPanic(err, "Failed to create new TaskContext")
	return ctx, control
}

// NewTestEnvironment creates a new Environment suitable for use in tests.
//
// This function should only be used in testing
func newTestEnvironment() *runtime.Environment {
	storage, err := runtime.NewTemporaryStorage(os.TempDir())
	nilOrPanic(err, "Failed to create temporary storage at: ", os.TempDir())

	folder, err := storage.NewFolder()
	nilOrPanic(err, "Failed to create temporary storage folder")

	// Set finalizer so that we always get the temporary folder removed.
	// This is should really only be used in tests, otherwise it would better to
	// call Remove() manually.
	rt.SetFinalizer(folder, func(f runtime.TemporaryFolder) {
		f.Remove()
	})

	return &runtime.Environment{
		GarbageCollector: &gc.GarbageCollector{},
		TemporaryStorage: folder,
		Monitor:          mocks.NewMockMonitor(true),
	}
}

func parseTestPayload(engine engines.Engine, payload string) map[string]interface{} {
	var jsonPayload map[string]interface{}
	err := json.Unmarshal([]byte(payload), &jsonPayload)
	nilOrPanic(err, "Payload parsing failed: ", payload)
	err = engine.PayloadSchema().Validate(jsonPayload)
	nilOrPanic(err, "Payload validation failed: ", payload, "\nError: ", err)
	return jsonPayload
}

func buildRunSandbox(b engines.SandboxBuilder) bool {
	// Start sandbox and wait for result
	sandbox, err := b.StartSandbox()
	nilOrPanic(err, "Failed to start sandbox")

	// Wait for result
	resultSet, err := sandbox.WaitForResult()
	nilOrPanic(err, "WaitForResult failed")

	// Get result and dispose ResultSet
	defer evalNilOrPanic(resultSet.Dispose, "Failed to dispose of ResultSet")
	return resultSet.Success()
}

// run helper
type run struct {
	provider       *EngineProvider
	context        *runtime.TaskContext
	control        *runtime.TaskContextController
	sandboxBuilder engines.SandboxBuilder
	sandbox        engines.Sandbox
	resultSet      engines.ResultSet
	logReader      io.ReadCloser
	closedLog      bool
	cleanup        func()
}

func (p *EngineProvider) newRun() *run {
	var cleanup func()
	if p.Setup != nil {
		cleanup = p.Setup()
	}
	p.ensureEngine()
	r := run{}
	r.cleanup = cleanup
	r.provider = p
	r.context, r.control = p.newTestTaskContext()
	return &r
}

func (r *run) NewSandboxBuilder(payload string) {
	assert(r.sandboxBuilder == nil, "NewSandboxBuilder already called!")
	sandboxBuilder, err := r.provider.engine.NewSandboxBuilder(engines.SandboxOptions{
		TaskContext: r.context,
		Payload:     parseTestPayload(r.provider.engine, payload),
		Monitor:     mocks.NewMockMonitor(true),
	})
	r.sandboxBuilder = sandboxBuilder
	nilOrPanic(err, "Error creating SandboxBuilder")
}

func (r *run) StartSandbox() {
	assert(r.sandbox == nil, "StartSandbox already called!")
	sandbox, err := r.sandboxBuilder.StartSandbox()
	nilOrPanic(err, "Failed to start sandbox")
	r.sandbox = sandbox
}

func (r *run) WaitForResult() bool {
	assert(r.resultSet == nil, "WaitForResult already called!")
	resultSet, err := r.sandbox.WaitForResult()
	nilOrPanic(err, "WaitForResult failed")
	r.resultSet = resultSet
	r.closedLog = true
	nilOrPanic(r.control.CloseLog(), "Failed to close log writer")
	return r.resultSet.Success()
}

func (r *run) OpenLogReader() {
	assert(r.logReader == nil, "OpenLogReader already called!")
	logReader, err := r.context.NewLogReader()
	nilOrPanic(err, "Failed to open log reader")
	r.logReader = logReader
}

func (r *run) ReadLog() string {
	reader, err := r.context.NewLogReader()
	nilOrPanic(err, "Failed to open log reader")
	defer reader.Close()
	data, err := ioutil.ReadAll(reader)
	nilOrPanic(err, "Failed to read log")
	return string(data)
}

func (r *run) Dispose() {
	// Make sure to call whatever cleanup routine was returned from setup
	defer func() {
		if r.provider != nil {
			r.provider.releaseEngine()
			r.provider = nil
		}

		if r.cleanup != nil {
			r.cleanup()
			r.cleanup = nil
		}
	}()
	if r.sandboxBuilder != nil {
		nilOrPanic(r.sandboxBuilder.Discard(), "")
		r.sandboxBuilder = nil
	}
	if r.sandbox != nil {
		err := r.sandbox.Abort()
		// Ensure the ResultSet is disposed too
		if err == engines.ErrSandboxTerminated && r.resultSet == nil {
			r.resultSet, _ = r.sandbox.WaitForResult()
		}
		if err != nil && err != engines.ErrSandboxTerminated {
			log.Panic("Sandbox.Abort() failed, error: ", err)
		}
		r.sandbox = nil
	}
	if r.resultSet != nil {
		nilOrPanic(r.resultSet.Dispose(), "")
		r.resultSet = nil
	}
	if r.logReader != nil {
		nilOrPanic(r.logReader.Close(), "")
		r.logReader = nil
	}
	if !r.closedLog {
		r.closedLog = true
		nilOrPanic(r.control.CloseLog(), "Failed to close log writer")
	}
	nilOrPanic(r.control.Dispose(), "")
}

// Auxiliary composite methods

func (r *run) buildRunSandbox() bool {
	r.StartSandbox()
	return r.WaitForResult()
}

func (r *run) GrepLog(needle string) bool {
	return strings.Contains(r.ReadLog(), needle)
}
