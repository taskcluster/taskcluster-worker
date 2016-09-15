package interactive

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	vnc "github.com/mitchellh/go-vnc"
	"github.com/stretchr/testify/mock"
	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-client-go/queue"
	"github.com/taskcluster/taskcluster-client-go/tcclient"
	"github.com/taskcluster/taskcluster-worker/plugins/interactive/displayclient"
	"github.com/taskcluster/taskcluster-worker/plugins/interactive/shellclient"
	"github.com/taskcluster/taskcluster-worker/plugins/plugintest"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

// expectS3Artifact will setup queue to expect an S3 artifact with given
// name to be created for taskID and runID using q and returns
// a channel which will receive the artifact.
func expectS3Artifact(q *client.MockQueue, taskID string, runID int, name string) <-chan []byte {
	c := make(chan []byte, 1)
	var s *httptest.Server
	s = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d, err := ioutil.ReadAll(r.Body)
		if err != nil {
			close(c)
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(200)
		c <- d
		go s.Close() // Close when all requests are done (don't block the request)
	}))
	data, _ := json.Marshal(queue.S3ArtifactResponse{
		StorageType: "s3",
		PutURL:      s.URL,
		ContentType: "application/octet",
		Expires:     tcclient.Time(time.Now().Add(30 * time.Minute)),
	})
	result := queue.PostArtifactResponse(data)
	q.On(
		"CreateArtifact",
		taskID, fmt.Sprintf("%d", runID),
		name, client.AnyPostArtifactRequest,
	).Return(&result, nil)
	return c
}

// expectRedirectArtifact will setup q to expect a redirect artifact with given
// name for taskID and runID to be created. This function returns a channel for
// the url of the redirect artifact.
func expectRedirectArtifact(q *client.MockQueue, taskID string, runID int, name string) <-chan string {
	c := make(chan string, 1)
	data, _ := json.Marshal(queue.RedirectArtifactResponse{
		StorageType: "reference",
	})
	result := queue.PostArtifactResponse(data)
	q.On(
		"CreateArtifact",
		taskID, fmt.Sprintf("%d", runID),
		name, client.AnyPostArtifactRequest,
	).Run(func(args mock.Arguments) {
		d := args.Get(3).(*queue.PostArtifactRequest)
		var r queue.RedirectArtifactRequest
		if json.Unmarshal(*d, &r) != nil {
			close(c)
			return
		}
		c <- r.URL
	}).Return(&result, nil)

	return c
}

type resolution struct {
	width  int
	height int
}

func getDisplayResolution(c io.ReadWriteCloser) (resolution, error) {
	client, err := vnc.Client(ioext.NopConn(c), &vnc.ClientConfig{})
	if err != nil {
		return resolution{}, err
	}
	client.Close()
	return resolution{
		width:  int(client.FrameBufferWidth),
		height: int(client.FrameBufferHeight),
	}, nil
}

func TestInteractivePluginDoingNothing(*testing.T) {
	plugintest.Case{
		Payload: `{
			"delay": 250,
			"function": "true",
			"argument": "whatever"
		}`,
		Plugin:        "interactive",
		PluginConfig:  `{}`,
		PluginSuccess: true,
		EngineSuccess: true,
	}.Test()
}

func TestInteractivePluginShell(t *testing.T) {
	taskID := slugid.V4()
	q := &client.MockQueue{}
	shell := expectRedirectArtifact(q, taskID, 0, "private/interactive/shell.html")
	sockets := expectS3Artifact(q, taskID, 0, "private/interactive/sockets.json")
	plugintest.Case{
		Payload: `{
			"delay": 250,
			"function": "true",
			"argument": "whatever",
			"interactive": {
				"disableDisplay": true
			}
		}`,
		Plugin:        "interactive",
		PluginConfig:  `{}`,
		PluginSuccess: true,
		EngineSuccess: true,
		QueueMock:     q,
		TaskID:        taskID,
		AfterStarted: func(plugintest.Options) {
			shellToolURL := <-shell
			u, _ := url.Parse(shellToolURL)
			shellSocketURL := u.Query().Get("socketUrl")

			// Check that socket.json contains the socket url too
			var s map[string]string
			json.Unmarshal(<-sockets, &s)
			if shellSocketURL != s["shellSocketUrl"] {
				panic("Expected shellSocketUrl to match redirect artifact target")
			}

			debug("Opening a new shell")
			sh, err := shellclient.Dial(shellSocketURL, nil, false)
			if err != nil {
				panic(fmt.Sprintf("Failed to open shell, error: %s", err))
			}

			debug("Write print-hello to shell")
			go func() {
				sh.StdinPipe().Write([]byte("print-hello"))
				sh.StdinPipe().Close()
			}()

			debug("Read message from shell")
			msg, err := ioutil.ReadAll(sh.StdoutPipe())
			if err != nil {
				panic(fmt.Sprintf("Error reading from shell, error: %s", err))
			}
			if string(msg) != "Hello World" {
				panic(fmt.Sprintf("Expected 'Hello World' got: '%s'", string(msg)))
			}

			debug("Wait for shell to terminate")
			result, err := sh.Wait()
			if err != nil {
				panic(fmt.Sprintf("Error from shell, error: %s", err))
			}
			if !result {
				panic("Shell didn't end successfully")
			}
		},
	}.Test()
}

func TestInteractivePluginDisplay(t *testing.T) {
	taskID := slugid.V4()
	q := &client.MockQueue{}
	display := expectRedirectArtifact(q, taskID, 0, "private/interactive/display.html")
	sockets := expectS3Artifact(q, taskID, 0, "private/interactive/sockets.json")
	plugintest.Case{
		Payload: `{
			"delay": 250,
			"function": "true",
			"argument": "whatever",
			"interactive": {
				"disableShell": true
			}
		}`,
		Plugin:        "interactive",
		PluginConfig:  `{}`,
		PluginSuccess: true,
		EngineSuccess: true,
		QueueMock:     q,
		TaskID:        taskID,
		AfterStarted: func(plugintest.Options) {
			displayToolURL := <-display
			u, _ := url.Parse(displayToolURL)
			displaysURL := u.Query().Get("displaysUrl")
			socketURL := u.Query().Get("socketUrl")

			// Check that socket.json contains the socket url too
			var s map[string]string
			json.Unmarshal(<-sockets, &s)
			if socketURL != s["displaySocketUrl"] {
				panic("Expected displaySocketUrl to match redirect artifact target")
			}
			if displaysURL != s["displaysUrl"] {
				panic("Expected displaySocketUrl to match redirect artifact target")
			}

			debug("List displays")
			displays, err := displayclient.ListDisplays(displaysURL)
			if err != nil {
				panic(fmt.Sprintf("ListDisplays failed, error: %s", err))
			}
			if len(displays) != 1 {
				panic("Expected ListDisplays to return at-least one display")
			}

			debug("OpenDisplay")
			d, err := displays[0].OpenDisplay()
			if err != nil {
				panic(fmt.Sprintf("Failed to OpenDisplay, error: %s", err))
			}

			debug("Get resolution")
			res, err := getDisplayResolution(d)
			if err != nil {
				panic(fmt.Sprintf("Failed connect to VNC display, error: %s", err))
			}

			// Some simple sanity tests, we can rely on the fact that resolution
			// doesn't change because we're testing against mock engine.
			if res.height != displays[0].Height {
				panic("height mismatch")
			}
			if res.width != displays[0].Width {
				panic("width mismatch")
			}
		},
	}.Test()
}
