package scripttest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	goruntime "runtime"
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
		Tasks: workertest.Tasks([]workertest.Task{{
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
		}}),
	}.Test(t)
}

func TestGetUrl(t *testing.T) {
	if goruntime.GOOS == "windows" {
		// On windows this fails with:
		// dial tcp 127.0.0.1:49523: socket: The requested service provider could not be loaded or initialized.
		// Calling the URL from this process works, but the subprocess fails...
		t.Skip("TODO: Fix the GetUrl on windows, probably firewall interference")
	}

	var u string
	workertest.Case{
		Engine:       "script",
		Concurrency:  1,
		EngineConfig: engineConfig,
		PluginConfig: pluginConfig,
		Setup: func(t *testing.T, env workertest.Environment) func() {
			// Create test server with magic-words, we'll grep for later
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`magic-words`))
			}))

			// Get url to testserver
			data, err := json.Marshal(s.URL)
			require.NoError(t, err)
			u = string(data)

			return func() {
				s.Close()
			}
		},
		Tasks: func(t *testing.T, env workertest.Environment) []workertest.Task {
			return []workertest.Task{{
				Title:           "read from url",
				Success:         true,
				Payload:         `{"result": "pass", "message": "hello-world", "url": ` + u + `}`,
				AllowAdditional: false,
				Artifacts: workertest.ArtifactAssertions{
					"public/logs/live.log":         workertest.ReferenceArtifact(),
					"public/logs/live_backing.log": workertest.GrepArtifact("magic-words"),
				},
			}}
		},
	}.Test(t)
}
