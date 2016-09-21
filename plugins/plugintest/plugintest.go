package plugintest

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	rt "runtime"
	"testing"

	"github.com/taskcluster/slugid-go/slugid"

	"github.com/taskcluster/taskcluster-worker/engines"
	// Ensure we load the mock engine
	_ "github.com/taskcluster/taskcluster-worker/engines/mock"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
	"github.com/taskcluster/taskcluster-worker/runtime/gc"
	"github.com/taskcluster/taskcluster-worker/runtime/webhookserver"
)

// The Options contains options available to any of the
// Before* or After* functions in plugintest.Case
type Options struct {
	Environment    *runtime.Environment
	SandboxBuilder engines.SandboxBuilder
	Engine         engines.Engine
	ResultSet      engines.ResultSet
	Plugin         plugins.Plugin
	TaskPlugin     plugins.TaskPlugin
}

// The Case is a testcase for a plugin. This specifies a few ways to
// ensure that the plugin has done what is expected. It
// works very closely with mockengine and using the functions
// part of the payload to modify how that works is very useful.
type Case struct {
	// A payload that will be passed to the sandbox and plugin
	Payload string
	// The plugin under test. This should be the name that is registered
	Plugin string
	// JSON configuration for the plugin
	PluginConfig string
	// Whether or not plugin.Stopped() should return true
	PluginSuccess bool
	// Whether or not engine.ResultSet.Success() should return true
	EngineSuccess bool
	// If a regular expression is specified here, it must be in the sandbox log
	MatchLog string
	// If a regular expression is specified here, it must _not_ be in the sandbox log
	NotMatchLog string
	// A mocked out queue client
	QueueMock *client.MockQueue
	// Override the default generated TaskID
	TaskID string
	// Override the default generated TaskID
	RunID int
	// A testing struct can be useful inside for assertions
	TestStruct *testing.T

	// Each of these functions is called at the time specified in the name
	BeforeBuildSandbox func(Options)
	AfterBuildSandbox  func(Options)
	BeforeStarted      func(Options)
	AfterStarted       func(Options)
	BeforeStopped      func(Options)
	AfterStopped       func(Options)
	BeforeFinished     func(Options)
	AfterFinished      func(Options)
	BeforeDisposed     func(Options)
	AfterDisposed      func(Options)
}

// Test is called to trigger a plugintest.Case to run
func (c Case) Test() {
	runtimeEnvironment := newTestEnvironment()

	testServer, err := webhookserver.NewTestServer()
	nilOrPanic(err)
	defer testServer.Stop()
	runtimeEnvironment.WebHookServer = testServer

	engineProvider := engines.Engines()["mock"]
	engine, err := engineProvider.NewEngine(engines.EngineOptions{
		Environment: runtimeEnvironment,
		Log:         runtimeEnvironment.Log.WithField("engine", "mock"),
		// TODO: Add engine config
	})
	nilOrPanic(err, "engineProvider.NewEngine failed")

	taskID := c.TaskID
	if taskID == "" {
		taskID = slugid.Nice()
	}

	context, controller, err := runtime.NewTaskContext(runtimeEnvironment.TemporaryStorage.NewFilePath(), runtime.TaskInfo{
		TaskID: taskID,
		RunID:  c.RunID,
	}, testServer)
	nilOrPanic(err)

	if c.QueueMock != nil {
		controller.SetQueueClient(c.QueueMock)
	}

	sandboxBuilder, err := engine.NewSandboxBuilder(engines.SandboxOptions{
		TaskContext: context,
		Payload:     parseEnginePayload(engine, c.Payload),
	})
	nilOrPanic(err, "engine.NewSandboxBuilder failed")

	provider := plugins.Plugins()[c.Plugin]
	assert(provider != nil, "Plugin does not exist! You tried to load: ", c.Plugin)
	p, err := provider.NewPlugin(plugins.PluginOptions{
		Environment: runtimeEnvironment,
		Engine:      engine,
		Log:         runtimeEnvironment.Log.WithField("plugin", c.Plugin),
		Config:      parsePluginConfig(provider, c.PluginConfig),
	})
	nilOrPanic(err, "pluginProvider.NewPlugin failed")

	tp, err := p.NewTaskPlugin(plugins.TaskPluginOptions{
		TaskInfo: &context.TaskInfo,
		Payload:  parsePluginPayload(p, c.Payload),
		Log:      runtimeEnvironment.Log.WithField("plugin", c.Plugin).WithField("taskId", taskID),
	})
	nilOrPanic(err, "plugin.NewTaskPlugin failed")
	// taskPlugin can be nil, if the plugin doesn't want any hooks
	if tp == nil {
		tp = plugins.TaskPluginBase{}
	}

	options := Options{
		Environment:    runtimeEnvironment,
		SandboxBuilder: sandboxBuilder,
		Engine:         engine,
		Plugin:         p,
		TaskPlugin:     tp,
	}

	err = tp.Prepare(context)
	nilOrPanic(err, "taskPlugin.Prepare failed")

	c.maybeRun(c.BeforeBuildSandbox, options)
	err = tp.BuildSandbox(sandboxBuilder)
	nilOrPanic(err, "taskPlugin.BuildSandbox failed")
	c.maybeRun(c.AfterBuildSandbox, options)

	c.maybeRun(c.BeforeStarted, options)
	sandbox, err := sandboxBuilder.StartSandbox()
	nilOrPanic(err, "sandboxBuilder.StartSandbox failed")
	err = tp.Started(sandbox)
	nilOrPanic(err, "taskPlugin.Started failed")
	c.maybeRun(c.AfterStarted, options)

	c.maybeRun(c.BeforeStopped, options)
	resultSet, err := sandbox.WaitForResult()
	nilOrPanic(err, "sandbox.WaitForResult failed")
	assert(resultSet.Success() == c.EngineSuccess)
	success, err := tp.Stopped(resultSet)
	nilOrPanic(err, "taskPlugin.Stopped failed")
	assert(success == c.PluginSuccess)
	c.maybeRun(c.AfterStopped, options)

	c.maybeRun(c.BeforeFinished, options)
	controller.CloseLog()
	err = tp.Finished(success)
	nilOrPanic(err, "taskPlugin.Finished failed")
	c.grepLog(context)
	c.maybeRun(c.AfterFinished, options)

	c.maybeRun(c.BeforeDisposed, options)
	controller.Dispose()
	err = tp.Dispose()
	nilOrPanic(err, "taskPlugin.Dispose failed")
	c.maybeRun(c.AfterDisposed, options)
}

func (c *Case) maybeRun(f func(Options), o Options) {
	if f != nil {
		f(o)
	}
}

func (c *Case) grepLog(context *runtime.TaskContext) {
	reader, err := context.NewLogReader()
	defer reader.Close()
	nilOrPanic(err, "Failed to open log reader")
	data, err := ioutil.ReadAll(reader)
	nilOrPanic(err, "Failed to read log")
	if c.MatchLog != "" {
		match, err := regexp.MatchString(c.MatchLog, string(data))
		nilOrPanic(err, "Invalid regular expression: ", c.MatchLog)
		assert(match, "Expected log to match regular expression: ", c.MatchLog)
	}
	if c.NotMatchLog != "" {
		match, err := regexp.MatchString(c.NotMatchLog, string(data))
		nilOrPanic(err, "Invalid regular expression: ", c.NotMatchLog)
		assert(!match, "Expected log to _not_ match regular expression: ", c.NotMatchLog)
	}
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

func parsePluginConfig(provider plugins.PluginProvider, data string) interface{} {
	if data == "" {
		return nil
	}
	var j interface{}
	err := json.Unmarshal([]byte(data), &j)
	nilOrPanic(err, "Failed to parse: ", data)
	err = provider.ConfigSchema().Validate(j)
	nilOrPanic(err, "Failed to validate against schema")
	return j
}

func parseEnginePayload(engine engines.Engine, payload string) map[string]interface{} {
	var jsonPayload map[string]interface{}
	err := json.Unmarshal([]byte(payload), &jsonPayload)
	nilOrPanic(err, "Payload parsing failed: ", payload)
	jsonPayload = engine.PayloadSchema().Filter(jsonPayload)
	err = engine.PayloadSchema().Validate(jsonPayload)
	nilOrPanic(err, "Payload validation failed: ", payload)
	return jsonPayload
}

func parsePluginPayload(plugin plugins.Plugin, payload string) map[string]interface{} {
	var jsonPayload map[string]interface{}
	err := json.Unmarshal([]byte(payload), &jsonPayload)
	nilOrPanic(err, "Payload parsing failed: ", payload)
	jsonPayload = plugin.PayloadSchema().Filter(jsonPayload)
	err = plugin.PayloadSchema().Validate(jsonPayload)
	nilOrPanic(err, "Payload validation failed: ", payload)
	return jsonPayload
}

func fmtPanic(a ...interface{}) {
	panic(fmt.Sprintln(a...))
}

func nilOrPanic(err error, a ...interface{}) {
	if err != nil {
		fmtPanic(append(a, err)...)
	}
}

func evalNilOrPanic(f func() error, a ...interface{}) {
	nilOrPanic(f(), a...)
}

func assert(condition bool, a ...interface{}) {
	if !condition {
		fmtPanic(a...)
	}
}
