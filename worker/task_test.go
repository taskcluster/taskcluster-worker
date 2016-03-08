package worker

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-client-go/queue"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/extpoints"
	"github.com/taskcluster/taskcluster-worker/plugins"
	pluginExtpoints "github.com/taskcluster/taskcluster-worker/plugins/extpoints"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

var logger, _ = runtime.CreateLogger(os.Getenv("LOGGING_LEVEL"))

type mockedPluginManager struct {
	payloadSchema      runtime.CompositeSchema
	payloadSchemaError error
	taskPlugin         plugins.TaskPlugin
	taskPluginError    error
}

func (m mockedPluginManager) PayloadSchema() (runtime.CompositeSchema, error) {
	return m.payloadSchema, m.payloadSchemaError
}

func (m mockedPluginManager) NewTaskPlugin(options plugins.TaskPluginOptions) (plugins.TaskPlugin, error) {
	return m.taskPlugin, m.taskPluginError
}

type plugin struct {
	plugins.PluginBase
}

type taskPlugin struct {
	plugins.TaskPluginBase
}

func ensureEnvironment(t *testing.T) (*runtime.Environment, engines.Engine, plugins.Plugin) {
	tempPath := filepath.Join(os.TempDir(), slugid.V4())
	tempStorage, err := runtime.NewTemporaryStorage(tempPath)
	if err != nil {
		t.Fatal(err)
	}

	environment := &runtime.Environment{
		TemporaryStorage: tempStorage,
	}
	engineProvider := extpoints.EngineProviders.Lookup("mock")
	engine, err := engineProvider.NewEngine(extpoints.EngineOptions{
		Environment: environment,
		Log:         logger.WithField("engine", "mock"),
	})
	if err != nil {
		t.Fatal(err.Error())
	}

	pluginOptions := &pluginExtpoints.PluginOptions{
		Environment: environment,
		Engine:      &engine,
		Log:         logger.WithField("component", "Plugin Manager"),
	}

	pm, err := pluginExtpoints.NewPluginManager([]string{"success"}, *pluginOptions)
	if err != nil {
		t.Fatalf("Error creating task manager. Could not create plugin manager. %s", err)
	}

	return environment, engine, pm
}

func TestParsePayload(t *testing.T) {
	var err error
	testCases := []struct {
		definition string
		shouldFail bool
	}{
		// Invalid JSON
		{definition: "", shouldFail: true},
		// Invalid Engine Payload
		{definition: `{"start": {"delay1": 10,"function": "write-log","argument": "Hello World"}}`, shouldFail: true},
		// Invalid Engine Payload
		{definition: `{"start": {"delay1": 10,"function": "write-log","argument": "Hello World"}}`, shouldFail: true},
		// Valid Engine Payload
		{definition: `{"start": {"delay": 10,"function": "write-log","argument": "Hello World"}}`, shouldFail: false},
	}

	environment, engine, pluginManager := ensureEnvironment(t)

	tr := &TaskRun{
		TaskID: "abc",
		RunID:  1,
		log:    logger.WithField("taskId", "abc"),
	}

	tp := environment.TemporaryStorage.NewFilePath()
	tr.context, tr.controller, err = runtime.NewTaskContext(tp)
	defer func() {
		tr.controller.CloseLog()
		tr.controller.Dispose()
	}()

	for _, tc := range testCases {
		tr.Definition = queue.TaskDefinitionResponse{
			Payload: []byte(tc.definition),
		}
		err = tr.ParsePayload(pluginManager, engine)
		assert.Equal(t, tc.shouldFail, err != nil, "Parsing invalid task payload should have returned an error")
	}
}

func TestCreateTaskPlugins(t *testing.T) {
	var err error
	environment, engine, pluginManager := ensureEnvironment(t)

	tr := &TaskRun{
		TaskID: "abc",
		RunID:  1,
		Definition: queue.TaskDefinitionResponse{
			Payload: []byte(`{"start": {"delay": 10,"function": "write-log","argument": "Hello World"}}`),
		},
		log: logger.WithField("taskId", "abc"),
	}

	tp := environment.TemporaryStorage.NewFilePath()
	tr.context, tr.controller, err = runtime.NewTaskContext(tp)

	err = tr.ParsePayload(pluginManager, engine)
	if err != nil {
		t.Fatal(err)
	}

	pm := mockedPluginManager{
		taskPlugin: &taskPlugin{},
	}

	err = tr.CreateTaskPlugins(pm)
	assert.Nil(t, err, "Error should not have been returned when creating task plugins")

	pm = mockedPluginManager{
		taskPlugin:      nil,
		taskPluginError: engines.NewMalformedPayloadError("bad payload"),
	}

	err = tr.CreateTaskPlugins(pm)
	assert.NotNil(t, err, "Error should have been returned when creating task plugins")
	assert.Equal(t, "engines.MalformedPayloadError", reflect.TypeOf(err).String())
}

func TestPrepare(t *testing.T) {
	var err error
	environment, engine, pluginManager := ensureEnvironment(t)

	tr := &TaskRun{
		TaskID: "abc",
		RunID:  1,
		Definition: queue.TaskDefinitionResponse{
			Payload: []byte(`{"start": {"delay": 10,"function": "write-log","argument": "Hello World"}}`),
		},
		log:    logger.WithField("taskId", "abc"),
		plugin: &taskPlugin{},
		engine: engine,
	}

	tp := environment.TemporaryStorage.NewFilePath()
	tr.context, tr.controller, err = runtime.NewTaskContext(tp)

	err = tr.ParsePayload(pluginManager, engine)
	assert.Nil(t, err)

	err = tr.CreateTaskPlugins(pluginManager)
	assert.Nil(t, err)

	err = tr.PrepareStage()
	assert.Nil(t, err)
}
