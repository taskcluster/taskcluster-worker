package taskrun

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/mock"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
	"github.com/taskcluster/taskcluster-worker/runtime/gc"
	"github.com/taskcluster/taskcluster-worker/runtime/mocks"
	"github.com/taskcluster/taskcluster-worker/runtime/webhookserver"
)

func TestTaskRun(t *testing.T) {
	// Setup an environment
	storage := runtime.NewTemporaryTestFolderOrPanic()
	defer storage.Remove()
	server, _ := webhookserver.NewTestServer()
	defer server.Stop()
	gc := gc.New("", 0, 0)
	defer gc.CollectAll()
	env := runtime.Environment{
		Monitor:          mocks.NewMockMonitor(true),
		GarbageCollector: gc,
		TemporaryStorage: storage,
		WebHookServer:    server,
	}

	options := Options{
		Environment: env,
		Engine: mockengine.New(engines.EngineOptions{
			Environment: &env,
			Monitor:     env.Monitor.WithPrefix("engine"),
		}),
		Plugin:  nil, // overwritten in each test case
		Monitor: env.Monitor.WithPrefix("taskrun"),
		TaskInfo: runtime.TaskInfo{
			TaskID: "--test-task-id--",
			RunID:  0,
		},
		Payload: nil, // overwritten in each test case
		Queue:   &client.MockQueue{},
	}

	// To work around a stupidity in mock.Mock where it creates a string from
	// the values which causes a race condition we have to use AnythingOfType
	// See: https://github.com/stretchr/testify/issues/173
	var taskPluginOptions = mock.AnythingOfType("plugins.TaskPluginOptions")
	var mockSandbox = mock.AnythingOfType("*mockengine.sandbox")
	var mockSandboxBuilder = mock.AnythingOfType("*mockengine.sandbox")
	var mockResultSet = mock.AnythingOfType("*mockengine.sandbox")
	t.Run("success", func(t *testing.T) {
		plugin := &mockPlugin{}
		plugin.On("PayloadSchema").Return(schematypes.Object{})
		plugin.On("NewTaskPlugin", taskPluginOptions).Return(plugin, nil)
		plugin.On("BuildSandbox", mockSandboxBuilder).Return(nil)
		plugin.On("Started", mockSandbox).Return(nil)
		plugin.On("Stopped", mockResultSet).Return(func(result engines.ResultSet) bool {
			return result.Success()
		}, nil)
		plugin.On("Finished", true).Return(nil)
		plugin.On("Dispose").Return(nil)
		defer plugin.AssertExpectations(t)
		options.Plugin = plugin

		require.NoError(t, json.Unmarshal([]byte(`{
			"delay":    0,
			"function": "true",
			"argument": ""
		}`), &options.Payload), "unable to parse payload")

		run := New(options)
		success, exception, _ := run.WaitForResult()
		assert.True(t, success, "expected success to be true")
		assert.False(t, exception, "expected exception to be false")

		require.NoError(t, run.Dispose(), "run.Dispose() returned an error")
	})

	t.Run("success with delay", func(t *testing.T) {
		plugin := &mockPlugin{}
		plugin.On("PayloadSchema").Return(schematypes.Object{})
		plugin.On("NewTaskPlugin", taskPluginOptions).Return(plugin, nil)
		plugin.On("BuildSandbox", mockSandboxBuilder).Return(nil)
		plugin.On("Started", mockSandbox).Return(nil)
		plugin.On("Stopped", mockResultSet).Return(func(result engines.ResultSet) bool {
			return result.Success()
		}, nil)
		plugin.On("Finished", true).Return(nil)
		plugin.On("Dispose").Return(nil)
		defer plugin.AssertExpectations(t)
		options.Plugin = plugin

		require.NoError(t, json.Unmarshal([]byte(`{
			"delay":   10,
			"function": "true",
			"argument": ""
		}`), &options.Payload), "unable to parse payload")

		run := New(options)
		success, exception, _ := run.WaitForResult()
		assert.True(t, success, "expected success to be true")
		assert.False(t, exception, "expected exception to be false")

		require.NoError(t, run.Dispose(), "run.Dispose() returned an error")
	})

	t.Run("failed", func(t *testing.T) {
		plugin := &mockPlugin{}
		plugin.On("PayloadSchema").Return(schematypes.Object{})
		plugin.On("NewTaskPlugin", taskPluginOptions).Return(plugin, nil)
		plugin.On("BuildSandbox", mockSandboxBuilder).Return(nil)
		plugin.On("Started", mockSandbox).Return(nil)
		plugin.On("Stopped", mockResultSet).Return(func(result engines.ResultSet) bool {
			return result.Success()
		}, nil)
		plugin.On("Finished", false).Return(nil)
		plugin.On("Dispose").Return(nil)
		defer plugin.AssertExpectations(t)
		options.Plugin = plugin

		require.NoError(t, json.Unmarshal([]byte(`{
			"delay":    10,
			"function": "false",
			"argument": ""
		}`), &options.Payload), "unable to parse payload")

		run := New(options)
		success, exception, _ := run.WaitForResult()
		assert.False(t, success, "expected success to be false")
		assert.False(t, exception, "expected exception to be false")

		require.NoError(t, run.Dispose(), "run.Dispose() returned an error")
	})

	t.Run("malformed-payload initial", func(t *testing.T) {
		plugin := &mockPlugin{}
		plugin.On("PayloadSchema").Return(schematypes.Object{})
		plugin.On("NewTaskPlugin", taskPluginOptions).Return(plugin, nil)
		plugin.On("Exception", runtime.ReasonMalformedPayload).Return(nil)
		plugin.On("Dispose").Return(nil)
		defer plugin.AssertExpectations(t)
		options.Plugin = plugin

		require.NoError(t, json.Unmarshal([]byte(`{
			"delay":    10,
			"function": "malformed-payload-initial",
			"argument": "example of a bad payload"
		}`), &options.Payload), "unable to parse payload")

		run := New(options)
		success, exception, reason := run.WaitForResult()
		assert.False(t, success, "expected success to be false")
		assert.True(t, exception, "expected exception to be true")
		assert.Equal(t, runtime.ReasonMalformedPayload, reason, "expected malformed-payload")

		require.NoError(t, run.Dispose(), "run.Dispose() returned an error")
	})

	t.Run("malformed-payload after start", func(t *testing.T) {
		plugin := &mockPlugin{}
		plugin.On("PayloadSchema").Return(schematypes.Object{})
		plugin.On("NewTaskPlugin", taskPluginOptions).Return(plugin, nil)
		plugin.On("BuildSandbox", mockSandboxBuilder).Return(nil)
		plugin.On("Started", mockSandbox).Return(nil)
		plugin.On("Exception", runtime.ReasonMalformedPayload).Return(nil)
		plugin.On("Dispose").Return(nil)
		defer plugin.AssertExpectations(t)
		options.Plugin = plugin

		require.NoError(t, json.Unmarshal([]byte(`{
			"delay":    10,
			"function": "malformed-payload-after-start",
			"argument": "example of a bad payload"
		}`), &options.Payload), "unable to parse payload")

		run := New(options)
		success, exception, reason := run.WaitForResult()
		assert.False(t, success, "expected success to be false")
		assert.True(t, exception, "expected exception to be true")
		assert.Equal(t, runtime.ReasonMalformedPayload, reason, "expected malformed-payload")

		require.NoError(t, run.Dispose(), "run.Dispose() returned an error")
	})

	t.Run("fatal internal engine error", func(t *testing.T) {
		plugin := &mockPlugin{}
		plugin.On("PayloadSchema").Return(schematypes.Object{})
		plugin.On("NewTaskPlugin", taskPluginOptions).Return(plugin, nil)
		plugin.On("BuildSandbox", mockSandboxBuilder).Return(nil)
		plugin.On("Started", mockSandbox).Return(nil)
		plugin.On("Exception", runtime.ReasonInternalError).Return(nil)
		plugin.On("Dispose").Return(nil)
		defer plugin.AssertExpectations(t)
		options.Plugin = plugin

		require.NoError(t, json.Unmarshal([]byte(`{
			"delay":    10,
			"function": "fatal-internal-error",
			"argument": ""
		}`), &options.Payload), "unable to parse payload")

		run := New(options)
		success, exception, reason := run.WaitForResult()
		assert.False(t, success, "expected success to be false")
		assert.True(t, exception, "expected exception to be true")
		assert.Equal(t, runtime.ReasonInternalError, reason, "expected internal-error")

		err := run.Dispose()
		require.Equal(t, runtime.ErrFatalInternalError, err, "expected fatal error")
	})

	t.Run("non-fatal internal engine error", func(t *testing.T) {
		plugin := &mockPlugin{}
		plugin.On("PayloadSchema").Return(schematypes.Object{})
		plugin.On("NewTaskPlugin", taskPluginOptions).Return(plugin, nil)
		plugin.On("BuildSandbox", mockSandboxBuilder).Return(nil)
		plugin.On("Started", mockSandbox).Return(nil)
		plugin.On("Exception", runtime.ReasonInternalError).Return(nil)
		plugin.On("Dispose").Return(nil)
		defer plugin.AssertExpectations(t)
		options.Plugin = plugin

		require.NoError(t, json.Unmarshal([]byte(`{
			"delay":    10,
			"function": "nonfatal-internal-error",
			"argument": ""
		}`), &options.Payload), "unable to parse payload")

		run := New(options)
		success, exception, reason := run.WaitForResult()
		assert.False(t, success, "expected success to be false")
		assert.True(t, exception, "expected exception to be true")
		assert.Equal(t, runtime.ReasonInternalError, reason, "expected internal-error")

		err := run.Dispose()
		require.Equal(t, runtime.ErrNonFatalInternalError, err, "expected non-fatal error")
	})

	t.Run("fatal internal plugin error", func(t *testing.T) {
		plugin := &mockPlugin{}
		plugin.On("PayloadSchema").Return(schematypes.Object{})
		plugin.On("NewTaskPlugin", taskPluginOptions).Return(plugin, nil)
		plugin.On("BuildSandbox", mockSandboxBuilder).Return(nil)
		plugin.On("Started", mockSandbox).Return(runtime.ErrFatalInternalError)
		plugin.On("Exception", runtime.ReasonInternalError).Return(nil)
		plugin.On("Dispose").Return(nil)
		defer plugin.AssertExpectations(t)
		options.Plugin = plugin

		require.NoError(t, json.Unmarshal([]byte(`{
			"delay":    0,
			"function": "true",
			"argument": ""
		}`), &options.Payload), "unable to parse payload")

		run := New(options)
		success, exception, reason := run.WaitForResult()
		assert.False(t, success, "expected success to be false")
		assert.True(t, exception, "expected exception to be true")
		assert.Equal(t, runtime.ReasonInternalError, reason, "expected internal-error")

		err := run.Dispose()
		require.Equal(t, runtime.ErrFatalInternalError, err, "expected fatal error")
	})

	t.Run("non-fatal internal plugin error", func(t *testing.T) {
		plugin := &mockPlugin{}
		plugin.On("PayloadSchema").Return(schematypes.Object{})
		plugin.On("NewTaskPlugin", taskPluginOptions).Return(plugin, nil)
		plugin.On("BuildSandbox", mockSandboxBuilder).Return(nil)
		plugin.On("Started", mockSandbox).Return(runtime.ErrNonFatalInternalError)
		plugin.On("Exception", runtime.ReasonInternalError).Return(nil)
		plugin.On("Dispose").Return(nil)
		defer plugin.AssertExpectations(t)
		options.Plugin = plugin

		require.NoError(t, json.Unmarshal([]byte(`{
			"delay":    0,
			"function": "true",
			"argument": ""
		}`), &options.Payload), "unable to parse payload")

		run := New(options)
		success, exception, reason := run.WaitForResult()
		assert.False(t, success, "expected success to be false")
		assert.True(t, exception, "expected exception to be true")
		assert.Equal(t, runtime.ReasonInternalError, reason, "expected internal-error")

		err := run.Dispose()
		require.Equal(t, runtime.ErrNonFatalInternalError, err, "expected non-fatal error")
	})

	t.Run("Abort worker-shutdown", func(t *testing.T) {
		var run *TaskRun
		var ctx *runtime.TaskContext
		plugin := &mockPlugin{}
		plugin.On("PayloadSchema").Return(schematypes.Object{})
		plugin.On("NewTaskPlugin", taskPluginOptions).Return(plugin, func(options plugins.TaskPluginOptions) error {
			ctx = options.TaskContext
			return nil
		})
		plugin.On("BuildSandbox", mockSandboxBuilder).Return(nil)
		plugin.On("Started", mockSandbox).Return(func(engines.Sandbox) error {
			assert.NotNil(t, ctx, "Expected TaskContext to be present")
			assert.NoError(t, ctx.Err(), "TaskContext is already aborted!")
			<-time.After(5 * time.Millisecond)
			go run.Abort(WorkerShutdown)
			<-ctx.Done() // Wait for TaskContext to be resolved
			return nil
		})
		plugin.On("Exception", runtime.ReasonWorkerShutdown).Return(nil)
		plugin.On("Dispose").Return(nil)
		defer plugin.AssertExpectations(t)
		options.Plugin = plugin

		require.NoError(t, json.Unmarshal([]byte(`{
			"delay":    50,
			"function": "true",
			"argument": ""
		}`), &options.Payload), "unable to parse payload")

		run = New(options)
		success, exception, reason := run.WaitForResult()
		assert.False(t, success, "expected success to be false")
		assert.True(t, exception, "expected exception to be true")
		assert.Equal(t, runtime.ReasonWorkerShutdown, reason, "expected worker-shutdown")

		require.NoError(t, run.Dispose(), "run.Dispose() returned an error")
	})

	t.Run("Abort canceled", func(t *testing.T) {
		var run *TaskRun
		var ctx *runtime.TaskContext
		plugin := &mockPlugin{}
		plugin.On("PayloadSchema").Return(schematypes.Object{})
		plugin.On("NewTaskPlugin", taskPluginOptions).Return(plugin, func(options plugins.TaskPluginOptions) error {
			ctx = options.TaskContext
			return nil
		})
		plugin.On("BuildSandbox", mockSandboxBuilder).Return(nil)
		plugin.On("Started", mockSandbox).Return(func(engines.Sandbox) error {
			assert.NotNil(t, ctx, "Expected TaskContext to be present")
			assert.NoError(t, ctx.Err(), "TaskContext is already aborted!")
			<-time.After(5 * time.Millisecond)
			go run.Abort(TaskCanceled)
			<-ctx.Done() // Wait for TaskContext to be resolved
			return nil
		})
		plugin.On("Exception", runtime.ReasonCanceled).Return(nil)
		plugin.On("Dispose").Return(nil)
		defer plugin.AssertExpectations(t)
		options.Plugin = plugin

		require.NoError(t, json.Unmarshal([]byte(`{
			"delay":    50,
			"function": "true",
			"argument": ""
		}`), &options.Payload), "unable to parse payload")

		run = New(options)
		success, exception, reason := run.WaitForResult()
		assert.False(t, success, "expected success to be false")
		assert.True(t, exception, "expected exception to be true")
		assert.Equal(t, runtime.ReasonCanceled, reason, "expected canceled")

		require.NoError(t, run.Dispose(), "run.Dispose() returned an error")
	})
}
