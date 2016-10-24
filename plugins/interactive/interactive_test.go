package interactive

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"testing"

	vnc "github.com/mitchellh/go-vnc"
	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-worker/plugins/interactive/displayclient"
	"github.com/taskcluster/taskcluster-worker/plugins/interactive/shellclient"
	"github.com/taskcluster/taskcluster-worker/plugins/plugintest"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

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
	shell := q.ExpectRedirectArtifact(taskID, 0, "private/interactive/shell.html")
	sockets := q.ExpectS3Artifact(taskID, 0, "private/interactive/sockets.json")
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
	display := q.ExpectRedirectArtifact(taskID, 0, "private/interactive/display.html")
	sockets := q.ExpectS3Artifact(taskID, 0, "private/interactive/sockets.json")
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
