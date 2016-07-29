package volume

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/extpoints"
	_ "github.com/taskcluster/taskcluster-worker/engines/mock"
	"github.com/taskcluster/taskcluster-worker/plugins"
	pluginExtpoints "github.com/taskcluster/taskcluster-worker/plugins/extpoints"
	"github.com/taskcluster/taskcluster-worker/plugins/plugintest"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

var logger, _ = runtime.CreateLogger(os.Getenv("LOGGING_LEVEL"))

type persistentVolumeTestCase struct {
	plugintest.Case
}

func TestPersistentVolumeReuse(t *testing.T) {
	payload := `{
		"start": {
			"delay": 10,
			"function": "set-volume",
			"argument": "/home/worker"
		},
		"volumes": {
			"persistent": [
				{
					"mountPoint": "/home/worker",
					"name": "test-workspace"
				}
			]
		}
	}`

	environment, engine := ensureEnvironment(t)
	taskID := slugid.Nice()
	context, controller, err := runtime.NewTaskContext(environment.TemporaryStorage.NewFilePath(), runtime.TaskInfo{
		TaskID: taskID,
		RunID:  0,
	})
	assert.Nil(t, err, "Could not create task context. %s", err)

	sandboxBuilder, err := engine.NewSandboxBuilder(engines.SandboxOptions{
		TaskContext: context,
		Payload:     parseEnginePayload(t, engine, payload),
	})
	assert.Nil(t, err, "Could not create sandbox builder. %s", err)

	provider := pluginExtpoints.PluginProviders.Lookup("volume")
	assert.NotNil(t, provider, "Plugin does not exist!")

	p, err := provider.NewPlugin(pluginExtpoints.PluginOptions{
		Environment: environment,
		Engine:      engine,
		Log:         logger.WithField("plugin", "volume"),
	})
	assert.Nil(t, err, "Could not load volume plugin. %s", err)
	vm := p.(volumeManager)
	pp := vm.plugins["persistent"].(*persistentVolumePlugin)
	assert.Equal(t, 1, len(vm.plugins), "Number of loaded plugins is incorrect")

	tp, err := p.NewTaskPlugin(plugins.TaskPluginOptions{
		TaskInfo: &context.TaskInfo,
		Payload:  parsePluginPayload(t, p, payload),
	})
	assert.Nil(t, err, "Could not create task plugin", err)

	assert.Equal(t, 0, len(pp.volumes))
	err = tp.Prepare(context)
	assert.Nil(t, err, "Could not prepare plugin", err)
	assert.Equal(t, 1, len(pp.volumes))

	err = tp.BuildSandbox(sandboxBuilder)
	assert.Nil(t, err, "Could not build sandbox", err)

	sandbox, err := sandboxBuilder.StartSandbox()
	assert.Nil(t, err, "Could not start sandbox", err)

	err = tp.Started(sandbox)
	assert.Nil(t, err, "Error calling 'started' on volume plugin", err)

	resultSet, err := sandbox.WaitForResult()
	assert.Nil(t, err, "Error waiting for resultset to complete", err)
	assert.True(t, resultSet.Success())

	success, err := tp.Stopped(resultSet)
	assert.Nil(t, err, "Error calling 'stopped' on volume plugin", err)
	assert.True(t, success)

	controller.CloseLog()

	err = tp.Finished(success)
	assert.Nil(t, err, "Error calling 'finished' on volume plugin", err)

	controller.Dispose()
	err = tp.Dispose()
	assert.Nil(t, err, "Error calling 'dispose' on volume plugin", err)
	assert.Equal(t, 1, len(pp.volumes))

	// get-volume should reuse previous volume and data should be set to
	// true.  If volume is not reused, the data filed will be set to false
	// because of a new volume being used and cause test to fail.
	payload = `{
		"start": {
			"delay": 10,
			"function": "get-volume",
			"argument": "/home/worker"
		},
		"volumes": {
			"persistent": [
				{
					"mountPoint": "/home/worker",
					"name": "test-workspace"
				}
			]
		}
	}`

	taskID = slugid.Nice()
	context, controller, _ = runtime.NewTaskContext(environment.TemporaryStorage.NewFilePath(), runtime.TaskInfo{
		TaskID: taskID,
		RunID:  0,
	})

	sandboxBuilder, _ = engine.NewSandboxBuilder(engines.SandboxOptions{
		TaskContext: context,
		Payload:     parseEnginePayload(t, engine, payload),
	})

	tp, _ = p.NewTaskPlugin(plugins.TaskPluginOptions{
		TaskInfo: &context.TaskInfo,
		Payload:  parsePluginPayload(t, p, payload),
	})

	tp.Prepare(context)
	assert.Equal(t, 1, len(pp.volumes))

	tp.BuildSandbox(sandboxBuilder)
	sandbox, _ = sandboxBuilder.StartSandbox()
	tp.Started(sandbox)

	resultSet, _ = sandbox.WaitForResult()
	assert.True(t, resultSet.Success())

	success, _ = tp.Stopped(resultSet)
	assert.True(t, success)

	controller.CloseLog()
	tp.Finished(success)
	controller.Dispose()
	tp.Dispose()
	assert.Equal(t, 1, len(pp.volumes))
}

func parsePluginPayload(t *testing.T, plugin plugins.Plugin, payload string) interface{} {
	jsonPayload := map[string]json.RawMessage{}

	err := json.Unmarshal([]byte(payload), &jsonPayload)
	assert.Nil(t, err, "Payload parsing failed.", err)

	s, err := plugin.PayloadSchema()
	assert.Nil(t, err, "Payload schema loading failed", err)

	p, err := s.Parse(jsonPayload)
	assert.Nil(t, err, "Payload parsing failed.")

	return p
}
func parseEnginePayload(t *testing.T, engine engines.Engine, payload string) interface{} {
	jsonPayload := map[string]json.RawMessage{}

	err := json.Unmarshal([]byte(payload), &jsonPayload)
	assert.Nil(t, err, "Payload parsing failed")

	p, err := engine.PayloadSchema().Parse(jsonPayload)
	assert.Nil(t, err, "Payload validation failed. %s", err)

	return p
}

func ensureEnvironment(t *testing.T) (*runtime.Environment, engines.Engine) {
	tempPath := filepath.Join(os.TempDir(), slugid.Nice())
	tempStorage, err := runtime.NewTemporaryStorage(tempPath)
	assert.Nil(t, err, "Could not create temporary Storage, %s", err)

	environment := &runtime.Environment{
		TemporaryStorage: tempStorage,
	}

	engineProvider := extpoints.EngineProviders.Lookup("mock")
	assert.NotNil(t, engineProvider, "Could not load engine provider.")

	engine, err := engineProvider.NewEngine(extpoints.EngineOptions{
		Environment: environment,
		Log:         logger.WithField("engine", "mock"),
	})
	assert.Nil(t, err, "Could not create new mock engine", err)

	return environment, engine
}
