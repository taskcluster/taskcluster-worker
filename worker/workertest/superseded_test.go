package workertest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-client-go/tcqueue"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

func TestSuperseded(t *testing.T) {
	// This test aims to check if the superseded functionality works. As we can't
	// know which order the tasks will be claimed in. This is a little complicated
	// to test. We do it by using the task index to recall which one of them set
	// to be superseded. When the HTTP server gets called as supersederUrl and
	// we haven't returned a superseded task yet, we take the next available taskId
	// as superseded the current taskId. We store the index of the superseded task
	// as superseded.
	superseded := -1 // index of task that was superseded
	var c Case
	var m sync.Mutex
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.Lock() // lock access to superseded
		defer m.Unlock()

		// Get taskId from which this is being called
		taskID := r.URL.Query().Get("taskId")

		// Create array of superseding taskIds, always start with its own taskId
		supersedes := []string{taskID}

		// If we haven't made a superseded task yet we do that now
		if superseded == -1 {
			// Find index of this taskId
			for index, task := range c.Tasks {
				if task.TaskID == taskID {
					superseded = index
					break
				}
			}
			// Make first other taskId be superseding this taskId
			for _, task := range c.Tasks {
				if task.TaskID != taskID {
					supersedes = append(supersedes, task.TaskID)
					break // only add one
				}
			}
		}

		data, _ := json.Marshal(struct {
			Supersedes []string `json:"supersedes"`
		}{Supersedes: supersedes})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer s.Close()

	// Create a StatusAssertion for task with index i, this looks as superseded
	// and check the task status structure.
	verifyTask := func(i int) StatusAssertion {
		return func(t *testing.T, q *tcqueue.Queue, status tcqueue.TaskStatusStructure) {
			// Sanity check for status structure
			assert.Equal(t, c.Tasks[i].TaskID, status.TaskID, "Expected taskId to match")
			if i == superseded {
				// If this is the task that was superseded, we assume it was resolved as such
				runID := len(status.Runs) - 1
				assert.Equal(t, status.Runs[runID].ReasonResolved, runtime.ReasonSuperseded.String(), "expected superseded")
			} else {
				// If this task was not superseded it should be successful
				assert.Equal(t, "completed", status.State, "expected successful task")
			}
		}
	}

	// Define test case with 3 tasks, that all look the same.
	c = Case{
		Engine:       "mock",
		Concurrency:  1,
		EngineConfig: `{}`,
		PluginConfig: `{
			"disabled": [],
			"success": {}
		}`,
		EnableSuperseding: true,
		Tasks: []Task{
			{
				TaskID:      slugid.Nice(),
				Title:       "Task 0",
				IgnoreState: true,
				Payload: `{
					"supersederUrl": "` + s.URL + `",
					"delay": 50,
					"function": "true",
					"argument": ""
				}`,
				Status: verifyTask(0),
			}, {
				TaskID:      slugid.Nice(),
				Title:       "Task 1",
				IgnoreState: true,
				Payload: `{
					"supersederUrl": "` + s.URL + `",
					"delay": 50,
					"function": "true",
					"argument": ""
				}`,
				Status: verifyTask(1),
			}, {
				TaskID:      slugid.Nice(),
				Title:       "Task 2",
				IgnoreState: true,
				Payload: `{
					"supersederUrl": "` + s.URL + `",
					"delay": 50,
					"function": "true",
					"argument": ""
				}`,
				Status: verifyTask(2),
			},
		},
	}
	c.Test(t)
}
