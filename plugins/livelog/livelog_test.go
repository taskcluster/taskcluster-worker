package livelog

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-worker/plugins/plugintest"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
)

func TestLiveLogStreaming(t *testing.T) {
	taskID := slugid.V4()

	// Create a mock queue
	q := &client.MockQueue{}
	livelog := q.ExpectRedirectArtifact(taskID, 0, "public/logs/live.log")
	backing := q.ExpectS3Artifact(taskID, 0, "public/logs/live_backing.log")

	// Setup test case
	plugintest.Case{
		// We use the ping-proxy payload here. This way the mock-engine won't
		// finish until the proxy replies 200 OK. In the proxy handler we don't
		// reply OK, until we've read 'Pinging' from the livelog. Hence, we ensure
		// that we're able to read a partial livelog.
		Payload: `{
			"delay": 0,
			"function": "ping-proxy",
			"argument": "http://my-proxy/test"
		}`,
		Proxies: map[string]http.Handler{
			"my-proxy": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Get the livelog url
				u := <-livelog
				assert.Contains(t, u, "http", "Expected a redirect URL")

				// Attempt a HEAD request
				req, err := http.NewRequest("HEAD", u, nil)
				require.NoError(t, err)
				res, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				res.Body.Close()
				require.Equal(t, http.StatusOK, res.StatusCode)

				// Attempt a POST request
				req, err = http.NewRequest("POST", u, nil)
				require.NoError(t, err)
				res, err = http.DefaultClient.Do(req)
				require.NoError(t, err)
				res.Body.Close()
				require.Equal(t, http.StatusMethodNotAllowed, res.StatusCode)

				// Open request to livelog
				req, err = http.NewRequest("GET", u, nil)
				require.NoError(t, err)
				res, err = http.DefaultClient.Do(req)
				require.NoError(t, err)
				defer res.Body.Close()

				// Read 'Pinging' from livelog before continuing
				data := []byte{}
				for err == nil {
					d := []byte{0}
					var n int
					n, err = res.Body.Read(d)
					if n > 0 {
						data = append(data, d[0])
					}
					if strings.Contains(string(data), "Pinging") {
						break
					}
				}

				// Reply so that we can continue
				if strings.Contains(string(data), "Pinging") {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("[hello-world-yt5aqnur3]"))
				} else {
					w.WriteHeader(400)
					w.Write([]byte("Expected to see 'Pinging'"))
				}
			}),
		},
		Plugin:        "livelog",
		TestStruct:    t,
		PluginSuccess: true,
		EngineSuccess: true,
		MatchLog:      "[hello-world-yt5aqnur3]",
		TaskID:        taskID,
		QueueMock:     q,
		AfterFinished: func(plugintest.Options) {
			assert.Contains(t, <-livelog, taskID, "Expected an artifact URL containing the taskId")
			assert.Contains(t, string(<-backing), "[hello-world-yt5aqnur3]")
		},
	}.Test()
}
