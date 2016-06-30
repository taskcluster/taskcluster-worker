package worker

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-client-go/queue"
	"github.com/taskcluster/taskcluster-client-go/tcclient"
	"github.com/taskcluster/taskcluster-worker/config"
	"github.com/taskcluster/taskcluster-worker/engines/extpoints"
	_ "github.com/taskcluster/taskcluster-worker/plugins/artifacts"
	_ "github.com/taskcluster/taskcluster-worker/plugins/env"
	_ "github.com/taskcluster/taskcluster-worker/plugins/success"
	_ "github.com/taskcluster/taskcluster-worker/plugins/volume"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type MockQueueService struct {
	tasks []*TaskRun
}

func (q *MockQueueService) ClaimWork(ntasks int) []*TaskRun {
	return q.tasks
}

func TestTaskManagerRunTask(t *testing.T) {
	resolved := false
	var handler = func(w http.ResponseWriter, r *http.Request) {
		resolved = true
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		json.NewEncoder(w).Encode(&queue.TaskStatusResponse{})
	}

	s := httptest.NewServer(http.HandlerFunc(handler))
	defer s.Close()

	logger, _ := runtime.CreateLogger(os.Getenv("LOGGING_LEVEL"))
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

	cfg := &config.Config{
		Taskcluster: struct {
			Queue struct {
				URL string `json:"url,omitempty"`
			} `json:"queue,omitempty"`
		}{
			Queue: struct {
				URL string `json:"url,omitempty"`
			}{
				URL: s.URL,
			},
		},
	}

	tm, err := newTaskManager(cfg, engine, environment, logger.WithField("test", "TestTaskManagerRunTask"))
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
			Payload: []byte(`{"start": {"delay": 1,"function": "write-log","argument": "Hello World"}}`),
		},
	}
	tm.run(claim)
	assert.True(t, resolved, "Task was not resolved")
}

func TestCancelTask(t *testing.T) {
	resolved := false
	var handler = func(w http.ResponseWriter, r *http.Request) {
		resolved = true
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		json.NewEncoder(w).Encode(&queue.TaskStatusResponse{})
	}

	s := httptest.NewServer(http.HandlerFunc(handler))
	defer s.Close()

	logger, _ := runtime.CreateLogger(os.Getenv("LOGGING_LEVEL"))
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

	cfg := &config.Config{
		Taskcluster: struct {
			Queue struct {
				URL string `json:"url,omitempty"`
			} `json:"queue,omitempty"`
		}{
			Queue: struct {
				URL string `json:"url,omitempty"`
			}{
				URL: s.URL,
			},
		},
	}

	tm, err := newTaskManager(cfg, engine, environment, logger.WithField("test", "TestRunTask"))
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
			Payload: []byte(`{"start": {"delay": 5000,"function": "write-log","argument": "Hello World"}}`),
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

	var handler = func(w http.ResponseWriter, r *http.Request) {
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

	s := httptest.NewServer(http.HandlerFunc(handler))
	defer s.Close()

	logger, _ := runtime.CreateLogger(os.Getenv("LOGGING_LEVEL"))
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

	cfg := &config.Config{
		Taskcluster: struct {
			Queue struct {
				URL string `json:"url,omitempty"`
			} `json:"queue,omitempty"`
		}{
			Queue: struct {
				URL string `json:"url,omitempty"`
			}{
				URL: s.URL,
			},
		},
	}
	tm, err := newTaskManager(cfg, engine, environment, logger.WithField("test", "TestRunTask"))
	if err != nil {
		t.Fatal(err)
	}

	claims := []*taskClaim{
		&taskClaim{
			taskID: "abc",
			runID:  1,
			definition: &queue.TaskDefinitionResponse{
				Payload: []byte(`{"start": {"delay": 5000,"function": "write-log","argument": "Hello World"}}`),
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
		&taskClaim{
			taskID: "def",
			runID:  0,
			definition: &queue.TaskDefinitionResponse{
				Payload: []byte(`{"start": {"delay": 5000,"function": "write-log","argument": "Hello World"}}`),
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
				tm.run(claim)
				wg.Done()
			}(c)
		}
	}()

	time.Sleep(500 * time.Millisecond)
	assert.Equal(t, len(tm.RunningTasks()), 2)
	close(tm.done)
	tm.Stop()

	wg.Wait()
	assert.Equal(t, 0, len(tm.RunningTasks()))
	assert.Equal(t, int32(2), atomic.LoadInt32(&resCount))
}
