package worker

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-client-go"
	"github.com/taskcluster/taskcluster-client-go/queue"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	_ "github.com/taskcluster/taskcluster-worker/plugins/artifacts"
	_ "github.com/taskcluster/taskcluster-worker/plugins/env"
	_ "github.com/taskcluster/taskcluster-worker/plugins/livelog"
	_ "github.com/taskcluster/taskcluster-worker/plugins/success"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/gc"
	"github.com/taskcluster/taskcluster-worker/runtime/mocks"
	"github.com/taskcluster/taskcluster-worker/runtime/webhookserver"
)

type MockPlugin struct {
	plugins.PluginBase
}

func (MockPlugin) NewTaskPlugin(plugins.TaskPluginOptions) (plugins.TaskPlugin, error) {
	return plugins.TaskPluginBase{}, nil
}

type MockQueueService struct {
	tasks []*TaskRun
}

func (q *MockQueueService) ClaimWork(ntasks int) []*TaskRun {
	return q.tasks
}

func TestTaskManagerRunTask(t *testing.T) {
	resolved := false
	var serverURL string
	var handler = func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/artifacts/public/logs/live_backing.log") {
			json.NewEncoder(w).Encode(&queue.S3ArtifactResponse{
				PutURL: serverURL,
			})
			return
		}

		if strings.Contains(r.URL.Path, "/artifacts/public/logs/live.log") {
			json.NewEncoder(w).Encode(&queue.RedirectArtifactResponse{})
			return
		}

		if strings.Contains(r.URL.Path, "/task/abc/runs/1/completed") {
			resolved = true
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			json.NewEncoder(w).Encode(&queue.TaskStatusResponse{})
		}
	}

	s := httptest.NewServer(http.HandlerFunc(handler))
	serverURL = s.URL
	defer s.Close()

	tempPath := filepath.Join(os.TempDir(), slugid.Nice())
	tempStorage, err := runtime.NewTemporaryStorage(tempPath)
	if err != nil {
		t.Fatal(err)
	}

	localServer, err := webhookserver.NewLocalServer(
		[]byte{127, 0, 0, 1}, 60000,
		"", 0,
		"example.com",
		"",
		"",
		"",
		10*time.Minute,
	)
	if err != nil {
		t.Error(err)
	}

	gc := &gc.GarbageCollector{}
	environment := &runtime.Environment{
		GarbageCollector: gc,
		TemporaryStorage: tempStorage,
		WebHookServer:    localServer,
		Monitor:          mocks.NewMockMonitor(true),
	}
	engineProvider := engines.Engines()["mock"]
	engine, err := engineProvider.NewEngine(engines.EngineOptions{
		Environment: environment,
		Monitor:     mocks.NewMockMonitor(true),
	})
	if err != nil {
		t.Fatal(err.Error())
	}

	cfg := &configType{
		QueueBaseURL: serverURL,
	}

	tm, err := newTaskManager(cfg, engine, MockPlugin{}, environment, mocks.NewMockMonitor(true), gc)
	if err != nil {
		t.Fatal(err)
	}

	claim := &taskClaim{
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
			Payload: []byte(`{"delay": 1,"function": "write-log","argument": "Hello World"}`),
		},
	}
	tm.run(claim)
	assert.True(t, resolved, "Task was not resolved")
}

func TestCancelTask(t *testing.T) {
	resolved := false
	var serverURL string
	var handler = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")

		if strings.Contains(r.URL.Path, "/artifacts/public/logs/live_backing.log") {
			json.NewEncoder(w).Encode(&queue.S3ArtifactResponse{
				PutURL: serverURL,
			})
			return
		}

		if strings.Contains(r.URL.Path, "/artifacts/public/logs/live.log") {
			json.NewEncoder(w).Encode(&queue.RedirectArtifactResponse{})
			return
		}

		if strings.Contains(r.URL.Path, "/task/abc/runs/1/") {
			resolved = true
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			json.NewEncoder(w).Encode(&queue.TaskStatusResponse{})
		}
	}

	s := httptest.NewServer(http.HandlerFunc(handler))
	serverURL = s.URL
	defer s.Close()

	tempPath := filepath.Join(os.TempDir(), slugid.Nice())
	tempStorage, err := runtime.NewTemporaryStorage(tempPath)
	if err != nil {
		t.Fatal(err)
	}

	localServer, err := webhookserver.NewLocalServer(
		[]byte{127, 0, 0, 1}, 60000,
		"", 0,
		"example.com",
		"",
		"",
		"",
		10*time.Minute,
	)
	if err != nil {
		t.Error(err)
	}

	gc := &gc.GarbageCollector{}
	environment := &runtime.Environment{
		GarbageCollector: gc,
		TemporaryStorage: tempStorage,
		WebHookServer:    localServer,
		Monitor:          mocks.NewMockMonitor(true),
	}
	engineProvider := engines.Engines()["mock"]
	engine, err := engineProvider.NewEngine(engines.EngineOptions{
		Environment: environment,
		Monitor:     mocks.NewMockMonitor(true),
	})
	if err != nil {
		t.Fatal(err.Error())
	}

	cfg := &configType{
		QueueBaseURL: serverURL,
	}

	tm, err := newTaskManager(cfg, engine, MockPlugin{}, environment, mocks.NewMockMonitor(true), gc)
	assert.Nil(t, err)

	claim := &taskClaim{
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
			Payload: []byte(`{"delay": 5000,"function": "write-log","argument": "Hello World"}`),
		},
	}
	done := make(chan struct{})
	go func() {
		tm.run(claim)
		close(done)
	}()

	time.Sleep(500 * time.Millisecond)
	assert.Equal(t, len(tm.RunningTasks()), 1)
	tm.CancelTask("abc", 1)

	<-done
	assert.Equal(t, len(tm.RunningTasks()), 0)
	assert.False(t, resolved, "Worker should not have resolved a cancelled task")
}

func TestWorkerShutdown(t *testing.T) {
	var resCount int32
	var serverURL string

	var handler = func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/artifacts/public/logs/live_backing.log") {
			json.NewEncoder(w).Encode(&queue.S3ArtifactResponse{
				PutURL: serverURL,
			})
			return
		}

		if strings.Contains(r.URL.Path, "/artifacts/public/logs/live.log") {
			json.NewEncoder(w).Encode(&queue.RedirectArtifactResponse{})
			return
		}

		if strings.Contains(r.URL.Path, "exception") {
			var exception queue.TaskExceptionRequest
			err := json.NewDecoder(r.Body).Decode(&exception)
			// Ignore errors for now
			if err != nil {
				return
			}

			assert.Equal(t, "worker-shutdown", exception.Reason)
			atomic.AddInt32(&resCount, 1)

			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			json.NewEncoder(w).Encode(&queue.TaskStatusResponse{})
		}
	}

	s := httptest.NewServer(http.HandlerFunc(handler))
	serverURL = s.URL
	defer s.Close()

	tempPath := filepath.Join(os.TempDir(), slugid.Nice())
	tempStorage, err := runtime.NewTemporaryStorage(tempPath)
	if err != nil {
		t.Fatal(err)
	}

	localServer, err := webhookserver.NewLocalServer(
		[]byte{127, 0, 0, 1}, 60000,
		"", 0,
		"example.com",
		"",
		"",
		"",
		10*time.Minute,
	)
	if err != nil {
		t.Error(err)
	}

	gc := &gc.GarbageCollector{}
	environment := &runtime.Environment{
		GarbageCollector: gc,
		TemporaryStorage: tempStorage,
		WebHookServer:    localServer,
		Monitor:          mocks.NewMockMonitor(true),
	}
	engineProvider := engines.Engines()["mock"]
	engine, err := engineProvider.NewEngine(engines.EngineOptions{
		Environment: environment,
		Monitor:     mocks.NewMockMonitor(true),
	})
	if err != nil {
		t.Fatal(err.Error())
	}

	cfg := &configType{
		QueueBaseURL: serverURL,
	}
	tm, err := newTaskManager(cfg, engine, MockPlugin{}, environment, mocks.NewMockMonitor(true), gc)
	if err != nil {
		t.Fatal(err)
	}

	claims := []*taskClaim{
		{
			taskID: "abc",
			runID:  1,
			definition: &queue.TaskDefinitionResponse{
				Payload: []byte(`{"delay": 5000,"function": "write-log","argument": "Hello World"}`),
			},
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
		},
		{
			taskID: "def",
			runID:  0,
			definition: &queue.TaskDefinitionResponse{
				Payload: []byte(`{"delay": 5000,"function": "write-log","argument": "Hello World"}`),
			},
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
		},
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		for _, c := range claims {
			go func(claim *taskClaim) {
				defer wg.Done()
				tm.run(claim)
			}(c)
		}
	}()

	time.Sleep(500 * time.Millisecond)
	assert.Equal(t, len(tm.RunningTasks()), 2)
	close(tm.doneClaimingTasks)
	close(tm.doneExecutingTasks)
	tm.ImmediateStop()

	wg.Wait()
	assert.Equal(t, 0, len(tm.RunningTasks()))
	assert.Equal(t, int32(2), atomic.LoadInt32(&resCount))
}
