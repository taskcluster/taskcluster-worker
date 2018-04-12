package scripttest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/taskcluster/taskcluster-worker/worker/workertest"
)

func TestLogging(t *testing.T) {
	workertest.Case{
		Engine:       "script",
		Concurrency:  1,
		EngineConfig: engineConfig,
		PluginConfig: pluginConfig,
		Tasks: []workertest.Task{{
			Title:           "hello-world pass",
			Success:         true,
			Payload:         `{"result": "pass", "message": "hello-world"}`,
			AllowAdditional: false,
			Artifacts: workertest.ArtifactAssertions{
				"public/logs/live.log":         workertest.ReferenceArtifact(),
				"public/logs/live_backing.log": workertest.GrepArtifact("hello-world"),
			},
		}, {
			Title:           "hello-world missing",
			Success:         true,
			Payload:         `{"result": "pass", "message": "hello-not-world"}`,
			AllowAdditional: false,
			Artifacts: workertest.ArtifactAssertions{
				"public/logs/live.log":         workertest.ReferenceArtifact(),
				"public/logs/live_backing.log": workertest.NotGrepArtifact("hello-world"),
			},
		}, {
			Title:           "hello-world fail",
			Success:         false,
			Payload:         `{"result": "fail", "message": "hello-world"}`,
			AllowAdditional: false,
			Artifacts: workertest.ArtifactAssertions{
				"public/logs/live.log":         workertest.ReferenceArtifact(),
				"public/logs/live_backing.log": workertest.GrepArtifact("hello-world"),
			},
		}, {
			Title:           "hello-world delay and pass",
			Success:         true,
			Payload:         `{"result": "pass", "message": "hello-world", "delay": 50}`,
			AllowAdditional: false,
			Artifacts: workertest.ArtifactAssertions{
				"public/logs/live.log":         workertest.ReferenceArtifact(),
				"public/logs/live_backing.log": workertest.GrepArtifact("hello-world"),
			},
		}},
	}.Test(t)
}

func TestGetUrl(t *testing.T) {
	// Create test server with magic-words, we'll grep for later
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`magic-words`))
	}))
	defer s.Close()

	// HACK: this that the server actually works...
	debug("URL: '%s'", s.URL)
	res, err := http.Get(s.URL)
	require.NoError(t, err, "GET to URL failed")
	defer res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode, "expected 200 OK")

	// Get url to testserver
	u, err := json.Marshal(s.URL)
	require.NoError(t, err)

	workertest.Case{
		Engine:       "script",
		Concurrency:  1,
		EngineConfig: engineConfig,
		PluginConfig: pluginConfig,
		Tasks: []workertest.Task{{
			Title:           "read from url",
			Success:         true,
			Payload:         `{"result": "pass", "message": "hello-world", "url": ` + string(u) + `}`,
			AllowAdditional: false,
			Artifacts: workertest.ArtifactAssertions{
				"public/logs/live.log":         workertest.ReferenceArtifact(),
				"public/logs/live_backing.log": workertest.GrepArtifact("magic-words"),
			},
		}},
	}.Test(t)
}
