package worker

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-client-go/queue"
	"github.com/taskcluster/taskcluster-client-go/tcclient"
	"github.com/taskcluster/taskcluster-worker/config"
	"github.com/taskcluster/taskcluster-worker/engines/extpoints"
	_ "github.com/taskcluster/taskcluster-worker/plugins/success"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type MockQueueService struct {
	tasks []*TaskRun
}

func (q *MockQueueService) ClaimWork(ntasks int) []*TaskRun {
	return q.tasks
}

func TestRunTask(t *testing.T) {
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

	tm, err := newTaskManager(&config.Config{}, engine, environment, logger.WithField("test", "TestRunTask"))
	if err != nil {
		t.Fatal(err)
	}

	mockedQueue := &MockQueue{}
	mockedQueue.On(
		"ReportCompleted",
		"abc",
		"1",
	).Return(&queue.TaskStatusResponse{}, &tcclient.CallSummary{}, nil)

	claim := &taskClaim{
		QueueClient: mockedQueue,
		TaskID:      "abc",
		RunID:       1,
		Definition: queue.TaskDefinitionResponse{
			Payload: []byte(`{"start": {"delay": 10,"function": "write-log","argument": "Hello World"}}`),
		},
	}
	tm.run(claim)
	mockedQueue.AssertCalled(t, "ReportCompleted", "abc", "1")
}

func TestCancelTask(t *testing.T) {
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

	tm, err := newTaskManager(&config.Config{}, engine, environment, logger.WithField("test", "TestRunTask"))
	if err != nil {
		t.Fatal(err)
	}

	reason := &queue.TaskExceptionRequest{
		Reason: "worker-shutdown",
	}

	mockedQueue := &MockQueue{}
	mockedQueue.On(
		"ReportException",
		"abc",
		"1",
		reason,
	).Return(&queue.TaskStatusResponse{
		Status: queue.TaskStatusStructure{},
	}, &tcclient.CallSummary{}, nil)

	claim := &taskClaim{
		TaskID: "abc",
		RunID:  1,
		Definition: queue.TaskDefinitionResponse{
			Payload: []byte(`{"start": {"delay": 5000,"function": "write-log","argument": "Hello World"}}`),
		},
		QueueClient: mockedQueue,
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

	mockedQueue.AssertNotCalled(t, "ReportException", "abc", "1", reason)
}

func TestWorkerShutdown(t *testing.T) {
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

	tm, err := newTaskManager(&config.Config{}, engine, environment, logger.WithField("test", "TestRunTask"))
	if err != nil {
		t.Fatal(err)
	}

	reason := &queue.TaskExceptionRequest{
		Reason: "worker-shutdown",
	}

	mockedQueue := &MockQueue{}
	mockedQueue.On(
		"ReportException",
		"abc",
		"1",
		reason,
	).Return(&queue.TaskStatusResponse{
		Status: queue.TaskStatusStructure{},
	}, &tcclient.CallSummary{}, nil)
	mockedQueue.On(
		"ReportException",
		"def",
		"0",
		reason,
	).Return(&queue.TaskStatusResponse{
		Status: queue.TaskStatusStructure{},
	}, &tcclient.CallSummary{}, nil)

	claims := []*taskClaim{&taskClaim{
		TaskID: "abc",
		RunID:  1,
		Definition: queue.TaskDefinitionResponse{
			Payload: []byte(`{"start": {"delay": 5000,"function": "write-log","argument": "Hello World"}}`),
		},
		QueueClient: mockedQueue,
	},
		&taskClaim{
			TaskID: "def",
			RunID:  0,
			Definition: queue.TaskDefinitionResponse{
				Payload: []byte(`{"start": {"delay": 5000,"function": "write-log","argument": "Hello World"}}`),
			},
			QueueClient: mockedQueue,
		}}

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
	assert.Equal(t, len(tm.RunningTasks()), 0)

	mockedQueue.AssertCalled(t, "ReportException", "abc", "1", reason)
	mockedQueue.AssertCalled(t, "ReportException", "def", "0", reason)
}
