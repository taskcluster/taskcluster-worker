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
	// to test. We do it by using the taskId to recall which one of the tasks were
	// superseded. When the HTTP server gets called as supersederUrl and we haven't
	// returned a superseded task yet, we take the next available taskId as
	// superseding the current taskId. We store the taskId of the superseded task
	// in supersededTaskID
	var s *httptest.Server
	var taskIDs []string
	var supersededTaskID string

	// Define test case with 3 tasks, that all look the same.
	Case{
		Engine:       "mock",
		Concurrency:  1,
		EngineConfig: `{}`,
		PluginConfig: `{
			"disabled": [],
			"success": {}
		}`,
		EnableSuperseding: true,
		Setup: func(t *testing.T, env Environment) func() {
			// Reset state
			taskIDs = []string{
				slugid.Nice(),
				slugid.Nice(),
				slugid.Nice(),
			}
			supersededTaskID = ""

			// create superseded server
			var m sync.Mutex
			s = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				m.Lock() // lock access to superseded
				defer m.Unlock()

				// Get taskId from which this is being called
				taskID := r.URL.Query().Get("taskId")

				// Create array of superseding taskIds, always start with its own taskId
				supersedes := []string{taskID}

				// If we haven't made a superseded task yet we do that now
				if supersededTaskID == "" {
					supersededTaskID = taskID
					// Make first other taskId be superseding this taskId
					for _, ID := range taskIDs {
						if ID != taskID {
							supersedes = append(supersedes, ID)
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

			return func() {
				s.Close()
			}
		},
		Tasks: func(t *testing.T, env Environment) []Task {
			// Create a StatusAssertion that checks to ensure that supersededTaskID
			// was infact superseded
			verifyTask := func(t *testing.T, q *tcqueue.Queue, status tcqueue.TaskStatusStructure) {
				if status.TaskID == supersededTaskID {
					// If this is the task that was superseded, we assume it was resolved as such
					runID := len(status.Runs) - 1
					assert.Equal(t, status.Runs[runID].ReasonResolved, runtime.ReasonSuperseded.String(), "expected superseded")
				} else {
					// If this task was not superseded it should be successful
					assert.Equal(t, "completed", status.State, "expected successful task")
				}
			}

			return []Task{
				{
					TaskID:      taskIDs[0],
					Title:       "Task 0",
					IgnoreState: true,
					Payload: `{
						"supersederUrl": "` + s.URL + `",
						"delay": 50,
						"function": "true",
						"argument": ""
					}`,
					Status: verifyTask,
				}, {
					TaskID:      taskIDs[1],
					Title:       "Task 1",
					IgnoreState: true,
					Payload: `{
						"supersederUrl": "` + s.URL + `",
						"delay": 50,
						"function": "true",
						"argument": ""
					}`,
					Status: verifyTask,
				}, {
					TaskID:      taskIDs[2],
					Title:       "Task 2",
					IgnoreState: true,
					Payload: `{
						"supersederUrl": "` + s.URL + `",
						"delay": 50,
						"function": "true",
						"argument": ""
					}`,
					Status: verifyTask,
				},
			}
		},
	}.Test(t)
}
