package workertest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

func TestSuperseded(t *testing.T) {
	var c Case
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		taskID := r.URL.Query().Get("taskId")
		supersedes := []string{taskID}
		if c.Tasks[0].TaskID == taskID {
			supersedes = append(supersedes, c.Tasks[1].TaskID)
		}

		data, _ := json.Marshal(struct {
			Supersedes []string `json:"supersedes"`
		}{Supersedes: supersedes})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer s.Close()

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
				TaskID:    slugid.Nice(),
				Title:     "Task Superseded",
				Exception: runtime.ReasonSuperseded,
				Payload: `{
					"supersederUrl": "` + s.URL + `",
					"delay": 50,
					"function": "true",
					"argument": ""
				}`,
			}, {
				TaskID:  slugid.Nice(),
				Title:   "Task Success",
				Success: true,
				Payload: `{
					"supersederUrl": "` + s.URL + `",
					"delay": 50,
					"function": "true",
					"argument": ""
				}`,
			}, {
				TaskID:  slugid.Nice(),
				Title:   "Task Success",
				Success: true,
				Payload: `{
					"supersederUrl": "` + s.URL + `",
					"delay": 50,
					"function": "true",
					"argument": ""
				}`,
			},
		},
	}
	c.Test(t)
}
