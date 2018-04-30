package scripttest

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	goruntime "runtime"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"github.com/taskcluster/slugid-go/slugid"
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

func mustMarshalJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		panic(errors.Wrapf(err, "failed to marshal JSON for type: %T", v))
	}
	return string(data)
}

// TestLiveLog tests that livelog plugin will serve the log while the task is
// runnning, and that slow log readers don't block task execution.
//
// To test livelog we run a task and read the livelog while the task is running.
// This test case does this expecting the livelog to contain a message before
// task is resolved.
//
// To avoid making an extremely intermittent test case, this test case uses
// a test-server that will:
//  1. open the livelog and read until it has read the message
//  2. replies 200 with a response payload, once step (1) is done.
//  3. Execute one of the following behaviors:
//     * 'close' the livelog without reading the rest of it.
//     * 'read-all' of the livelog before closing it
//     * 'leak' the livelog request not closing it until task is resolved
//
// The test task will then:
//  1. Print the message to log
//  2. Ping the test-server and block waiting for the reply
//  3. Print the test-server reply, exiting successfully if response is 200.
//
// By greping the final log for response payload we ensure that our test-server
// did indeed reply to the test task. By not replying from the test-server
// until we've read the message from the livelog, we ensure that the livelog
// works. If livelog doesn't work this should deadlock, or request should
// timeout in the test task, instead of returning 200.
//
// To increase test coverage we shall try with a few different message sizes
// and behaviors for the test-server once it has replied. The behaviors
// ensures that a blocking livelog reader doesn't block the worker, or cause
// other undesired behavior in the worker.
func TestLiveLog(t *testing.T) {
	if goruntime.GOOS == "windows" {
		// On windows this fails with:
		// dial tcp 127.0.0.1:49523: socket: The requested service provider could not be loaded or initialized.
		// Calling the URL from this process works, but the subprocess fails...
		t.Skip("TODO: Fix the GetUrl on windows, probably firewall interference")
	}

	// Generate some messages that can be written log, so we can read them from livelog
	// Trying different message sizes tests the buffer slicing code a bit.
	randMessage := make([]byte, 1024*8+27)
	rand.Read(randMessage)
	messages := map[string]string{
		"special":   "@", // single special character
		"short":     "hello world, this is a slightly longer message, maybe that tests something more",
		"very-long": base64.StdEncoding.EncodeToString(randMessage),
	}

	// Generate some replies that can be written log, so we know that test-server was called
	randReply := make([]byte, 1024*8+27)
	rand.Read(randReply)
	replies := map[string]string{
		"special":   "*", // single special character
		"short":     "magic-words, this is a slightly longer reply, maybe that tests something more",
		"very-long": base64.StdEncoding.EncodeToString(randReply),
	}

	var s *httptest.Server
	workertest.Case{
		Engine:       "script",
		Concurrency:  5, // this test runs a lot of tasks, we better run them in parallel
		EngineConfig: engineConfig,
		PluginConfig: pluginConfig,
		Setup: func(t *testing.T, env workertest.Environment) func() {
			done := make(chan struct{})
			// Create test-server with magic-words, we'll grep for later
			var wg sync.WaitGroup
			s = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				taskID := r.URL.Query().Get("taskId")
				msg := messages[r.URL.Query().Get("msgSize")]
				behavior := r.URL.Query().Get("behavior")
				reply := replies[r.URL.Query().Get("replySize")]

				debug("Test server called by, taskID: %s", taskID)
				u, err := env.Queue.GetLatestArtifact_SignedURL(taskID, "public/logs/live.log", 5*time.Minute)
				require.NoError(t, err, "GetLatestArtifact_SignedURL failed")

				debug("Fetching live.log")
				res, err := http.Get(u.String())
				require.NoError(t, err, "failed to fetch live.log")

				// Read until we see msg
				var data []byte
				b := make([]byte, 1)
				for {
					n, rerr := res.Body.Read(b)
					require.NoError(t, rerr, "failed to read body")
					data = append(data, b[0:n]...)
					if bytes.Contains(data, []byte(msg)) {
						debug("Read special message from livelog")
						break
					}
				}

				// Signal that everything went well, and return reply.
				debug("Replying to unblock task, taskID: %s", taskID)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(reply))

				// Now do 'behavior' on the livelog, do so asynchronously, so we don't
				// block the response in this handler.
				wg.Add(1)
				go func() {
					defer wg.Done()
					defer debug("Livelog finished, taskID: %s", taskID)
					defer res.Body.Close()

					// run given livelog behavior
					switch behavior {
					case "close":
						return // this will close res.Body
					case "read-all":
						rest, rerr := ioutil.ReadAll(res.Body) // read the rest of the livelog
						require.NoError(t, rerr, "failed to read the rest of the livelog from %s", taskID)
						require.Contains(t, string(rest), reply, "expected the rest of the livelog to contain the reply")
					case "leak":
						<-done // wait until the cleanup() function is called
					default:
						t.Errorf("undefined livelog behavior given: '%s'", behavior)
					}
				}()
			}))

			// cleanup function to be called when tests are done
			return func() {
				close(done) // signal leaked requests to be closed, so that we cleanup
				wg.Wait()   // Wait all the asynchronous livelog reading functions to finish
				s.Close()   // Close test server
			}
		},
		Tasks: func(t *testing.T, env workertest.Environment) []workertest.Task {
			var tasks []workertest.Task
			// Create tasks that tries out each message size and behavior
			for _, behavior := range []string{"close", "read-all", "leak"} {
				for replySize, reply := range replies {
					for msgSize, msg := range messages {
						taskID := slugid.Nice()
						// Create a URL for test-server that encodes what it should do
						u := s.URL + "/?" + url.Values{
							"msgSize":   []string{msgSize},   // message it should read from livelog
							"behavior":  []string{behavior},  // behavior for livelog after reading message
							"taskId":    []string{taskID},    // taskID to read livelog for
							"replySize": []string{replySize}, // reply to print once message is read
						}.Encode()
						tasks = append(tasks, workertest.Task{
							TaskID: taskID,
							Title: fmt.Sprintf(
								"read %s message from livelog, print %s reply and %s livelog",
								msgSize, replySize, behavior,
							),
							Success:         true,
							Payload:         `{"result": "pass", "message": ` + mustMarshalJSON(msg) + `, "url": ` + mustMarshalJSON(u) + `}`,
							AllowAdditional: false,
							Artifacts: workertest.ArtifactAssertions{
								"public/logs/live.log":         workertest.ReferenceArtifact(),
								"public/logs/live_backing.log": workertest.GrepArtifact(reply),
							},
						})
					}
				}
			}
			return tasks
		},
	}.Test(t)
}
