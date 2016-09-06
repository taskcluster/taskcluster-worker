package worker

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-client-go/queue"
	"github.com/taskcluster/taskcluster-client-go/tcclient"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
)

var logger, _ = runtime.CreateLogger(os.Getenv("LOGGING_LEVEL"))

var taskDefinitions = map[string]struct {
	definition string
	success    bool
}{
	"invalidJSON": {
		definition: "",
		success:    false,
	},
	"invalidEnginePayload": {
		definition: `{"delay1": 10,"function": "write-log","argument": "Hello World"}`,
		success:    false,
	},
	"validEnginePayload": {
		definition: `{"delay": 10,"function": "write-log","argument": "Hello World"}`,
		success:    true,
	},
}

var claim = &taskClaim{
	taskID: "abc",
	runID:  1,
	taskClaim: &queue.TaskClaimResponse{
		Credentials: struct {
			AccessToken string `json:"accessToken"`
			Certificate string `json:"certificate"`
			ClientID    string `json:"clientId"`
		}{
			AccessToken: "123",
			ClientID:    "abc",
			Certificate: "",
		},
		TakenUntil: tcclient.Time(time.Now().Add(time.Minute * 5)),
	},
	definition: &queue.TaskDefinitionResponse{
		Payload: []byte(taskDefinitions["validEnginePayload"].definition),
	},
}

type mockedPluginManager struct {
	plugins.PluginBase
	taskPlugin      plugins.TaskPlugin
	taskPluginError error
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
	tempPath := filepath.Join(os.TempDir(), slugid.Nice())
	tempStorage, err := runtime.NewTemporaryStorage(tempPath)
	if err != nil {
		t.Fatal(err)
	}

	environment := &runtime.Environment{
		TemporaryStorage: tempStorage,
	}
	engineProvider := engines.Engines()["mock"]
	engine, err := engineProvider.NewEngine(engines.EngineOptions{
		Environment: environment,
		Log:         logger.WithField("engine", "mock"),
	})
	if err != nil {
		t.Fatal(err.Error())
	}

	pluginOptions := plugins.PluginOptions{
		Environment: environment,
		Engine:      engine,
		Log:         logger.WithField("component", "Plugin Manager"),
	}

	pm, err := plugins.Plugins()["success"].NewPlugin(pluginOptions)
	if err != nil {
		t.Fatalf("Error creating task manager. Could not create plugin manager. %s", err)
	}

	return environment, engine, pm
}

func TestRunTask(t *testing.T) {
	environment, engine, pluginManager := ensureEnvironment(t)
	tr, err := newTaskRun(&configType{}, claim, environment, engine, pluginManager, logger.WithField("test", "TestRunTask"))
	assert.Nil(t, err)

	mockedQueue := &client.MockQueue{}
	mockedQueue.On(
		"ReportCompleted",
		"abc",
		"1",
	).Return(&queue.TaskStatusResponse{}, nil)

	tr.controller.SetQueueClient(mockedQueue)

	tr.Run()
	mockedQueue.AssertCalled(t, "ReportCompleted", "abc", "1")
}

func TestRunMalformedEnginePayloadTask(t *testing.T) {
	claim.definition = &queue.TaskDefinitionResponse{
		Payload: []byte(taskDefinitions["invalidEnginePayload"].definition),
	}

	environment, engine, pluginManager := ensureEnvironment(t)
	tr, err := newTaskRun(&configType{}, claim, environment, engine, pluginManager, logger.WithField("test", "TestRunTask"))
	assert.Nil(t, err)

	mockedQueue := &client.MockQueue{}
	mockedQueue.On(
		"ReportException",
		"abc",
		"1",
		&queue.TaskExceptionRequest{Reason: "malformed-payload"},
	).Return(&queue.TaskStatusResponse{}, nil)

	tr.controller.SetQueueClient(mockedQueue)

	tr.Run()
	mockedQueue.AssertCalled(t, "ReportException", "abc", "1", &queue.TaskExceptionRequest{Reason: "malformed-payload"})
}

func TestReclaimTask(t *testing.T) {
	environment, engine, pluginManager := ensureEnvironment(t)
	claim.definition = &queue.TaskDefinitionResponse{
		Payload: []byte(taskDefinitions["validEnginePayload"].definition),
	}
	claim.taskClaim.TakenUntil = tcclient.Time(time.Now().Add(time.Millisecond * 4))

	reclaimEvents := 0
	var handler = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		switch r.URL.Path {
		case "/task/abc/runs/1/reclaim":
			reclaimEvents++
			json.NewEncoder(w).Encode(&queue.TaskReclaimResponse{
				Credentials: struct {
					AccessToken string `json:"accessToken"`
					Certificate string `json:"certificate"`
					ClientID    string `json:"clientId"`
				}{
					AccessToken: "4567890",
					ClientID:    "def",
					Certificate: "",
				},
				TakenUntil: tcclient.Time(time.Now().Add(time.Millisecond * 4)),
			})
		case "/task/abc/runs/1/completed":
			json.NewEncoder(w).Encode(&queue.TaskStatusResponse{})
		}
	}

	s := httptest.NewServer(http.HandlerFunc(handler))
	defer s.Close()

	cfg := &configType{
		QueueBaseURL: s.URL,
	}

	tr, err := newTaskRun(cfg, claim, environment, engine, pluginManager, logger.WithField("test", "TestRunTask"))
	assert.Nil(t, err)

	oldClient := tr.context.Queue()

	tr.Run()

	newClient := tr.context.Queue()
	assert.NotEqual(t, newClient, oldClient, "clients should not be the same after reclaim")

	assert.True(
		t,
		reclaimEvents >= 0 && reclaimEvents <= 3,
		fmt.Sprintf("Task should have been reclaimed at least 1 times and not more than 3, but was reclaimed %d times", reclaimEvents),
	)
}
