package metaservice

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins/interactive/shellconsts"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
)

func TestMetaService(t *testing.T) {
	// Create temporary storage
	storage, err := runtime.NewTemporaryStorage(os.TempDir())
	if err != nil {
		panic("Failed to create TemporaryStorage")
	}

	// Setup a new MetaService
	log := bytes.NewBuffer(nil)
	result := false
	var resolved atomics.Once
	s := New([]string{"bash", "-c", "whoami"}, make(map[string]string), log, func(r bool) {
		if !resolved.Do(func() { result = r }) {
			panic("It shouldn't be possible to resolve twice")
		}
	}, &runtime.Environment{
		TemporaryStorage: storage,
	})

	// Upload some log
	req, err := http.NewRequest("POST", "http://169.254.169.254/engine/v1/log", bytes.NewBufferString("Hello World"))
	nilOrFatal(t, err)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	assert(t, w.Code == http.StatusOK)

	// Check the log
	if log.String() != "Hello World" {
		panic("Expected 'Hello World' in the log")
	}

	// Check that we can report success
	req, err = http.NewRequest("PUT", "http://169.254.169.254/engine/v1/success", nil)
	nilOrFatal(t, err)
	w = httptest.NewRecorder()
	s.ServeHTTP(w, req)
	assert(t, w.Code == http.StatusOK)

	// Check result
	resolved.Wait()
	assert(t, result)

	// Check idempotency
	req, err = http.NewRequest("PUT", "http://169.254.169.254/engine/v1/success", nil)
	nilOrFatal(t, err)
	w = httptest.NewRecorder()
	s.ServeHTTP(w, req)
	assert(t, w.Code == http.StatusOK)

	// Check that we can have a conflict
	req, err = http.NewRequest("PUT", "http://169.254.169.254/engine/v1/failed", nil)
	nilOrFatal(t, err)
	w = httptest.NewRecorder()
	s.ServeHTTP(w, req)
	assert(t, w.Code == http.StatusConflict)

	debug("### Test polling and get-artifact")

	// Check that we can poll for an action, and reply with an artifact
	go func() {
		// Start polling for an action
		req, err2 := http.NewRequest("GET", "http://169.254.169.254/engine/v1/poll", nil)
		nilOrFatal(t, err2)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, req)
		assert(t, w.Code == http.StatusOK)
		action := Action{}
		err2 = json.Unmarshal(w.Body.Bytes(), &action)
		nilOrFatal(t, err2, "Failed to decode JSON")

		// Check that the action is 'get-artifact' (as expected)
		assert(t, action.ID != "", "Expected action.ID != ''")
		assert(t, action.Type == "get-artifact", "Expected get-artifact action")
		assert(t, action.Path == "/home/worker/test-file", "Expected action.Path")

		// Post back an artifact
		req, err2 = http.NewRequest("POST",
			"http://169.254.169.254/engine/v1/reply?id="+action.ID,
			bytes.NewBufferString("hello-world"),
		)
		nilOrFatal(t, err2)
		w = httptest.NewRecorder()
		s.ServeHTTP(w, req)
		assert(t, w.Code == http.StatusOK)
	}()

	// Get artifact through metaservice
	f, err := s.GetArtifact("/home/worker/test-file")
	nilOrFatal(t, err, "Failed to get artifact")
	defer f.Close()
	b, err := ioutil.ReadAll(f)
	nilOrFatal(t, err, "Error reading from file")
	assert(t, string(b) == "hello-world", "Expected hello-world artifact")

	debug("### Test polling and get-artifact for non-existing file")

	// Check that we can poll for an action, and reply with an error not-found
	go func() {
		// Start polling for an action
		req, err2 := http.NewRequest("GET", "http://169.254.169.254/engine/v1/poll", nil)
		nilOrFatal(t, err2)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, req)
		assert(t, w.Code == http.StatusOK)
		action := Action{}
		err2 = json.Unmarshal(w.Body.Bytes(), &action)
		nilOrFatal(t, err2, "Failed to decode JSON")

		// Check that the action is 'get-artifact' (as expected)
		assert(t, action.ID != "", "Expected action.ID != ''")
		assert(t, action.Type == "get-artifact", "Expected get-artifact action")
		assert(t, action.Path == "/home/worker/wrong-file", "Expected action.Path")

		// Post back an artifact
		req, err2 = http.NewRequest("POST",
			"http://169.254.169.254/engine/v1/reply?id="+action.ID,
			nil,
		)
		nilOrFatal(t, err2)
		req.Header.Set("X-Taskcluster-Worker-Error", "file-not-found")
		w = httptest.NewRecorder()
		s.ServeHTTP(w, req)
		assert(t, w.Code == http.StatusOK)
	}()

	// Get error for artifact through metaservice
	f, err = s.GetArtifact("/home/worker/wrong-file")
	assert(t, f == nil, "Didn't expect to get a file")
	assert(t, err == engines.ErrResourceNotFound, "Expected ErrResourceNotFound")

	debug("### Test polling and list-folder")

	// Check that we can poll for an action, and reply to a list-folder
	go func() {
		// Start polling for an action
		req, err2 := http.NewRequest("GET", "http://169.254.169.254/engine/v1/poll", nil)
		nilOrFatal(t, err2)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, req)
		assert(t, w.Code == http.StatusOK)
		action := Action{}
		err2 = json.Unmarshal(w.Body.Bytes(), &action)
		nilOrFatal(t, err2, "Failed to decode JSON")

		// Check that the action is 'list-folder' (as expected)
		assert(t, action.ID != "", "Expected action.ID != ''")
		assert(t, action.Type == "list-folder", "Expected list-folder action")
		assert(t, action.Path == "/home/worker/", "Expected action.Path")

		// Post back an reply
		payload, _ := json.Marshal(Files{
			Files:    []string{"/home/worker/test-file"},
			NotFound: false,
		})
		req, err2 = http.NewRequest("POST",
			"http://169.254.169.254/engine/v1/reply?id="+action.ID,
			bytes.NewBuffer(payload),
		)
		nilOrFatal(t, err2)
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		s.ServeHTTP(w, req)
		assert(t, w.Code == http.StatusOK, "Unexpected status: ", w.Code)
	}()

	// List folder through metaservice
	files, err := s.ListFolder("/home/worker/")
	nilOrFatal(t, err, "Failed to list-folder")
	assert(t, len(files) == 1, "Expected one file")
	assert(t, files[0] == "/home/worker/test-file", "Got the wrong file")

	debug("### Test polling and list-folder (not-found)")

	// Check that we can poll for an action, and reply to a list-folder, not found
	go func() {
		// Start polling for an action
		req, err2 := http.NewRequest("GET", "http://169.254.169.254/engine/v1/poll", nil)
		nilOrFatal(t, err2)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, req)
		assert(t, w.Code == http.StatusOK)
		action := Action{}
		err2 = json.Unmarshal(w.Body.Bytes(), &action)
		nilOrFatal(t, err2, "Failed to decode JSON")

		// Check that the action is 'list-folder' (as expected)
		assert(t, action.ID != "", "Expected action.ID != ''")
		assert(t, action.Type == "list-folder", "Expected list-folder action")
		assert(t, action.Path == "/home/worker/missing/", "Expected action.Path")

		// Post back an reply
		payload, _ := json.Marshal(Files{
			Files:    nil,
			NotFound: true,
		})
		req, err2 = http.NewRequest("POST",
			"http://169.254.169.254/engine/v1/reply?id="+action.ID,
			bytes.NewBuffer(payload),
		)
		nilOrFatal(t, err2)
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		s.ServeHTTP(w, req)
		assert(t, w.Code == http.StatusOK, "Unexpected status: ", w.Code)
	}()

	// List folder through metaservice
	files, err = s.ListFolder("/home/worker/missing/")
	assert(t, err == engines.ErrResourceNotFound, "Expected ErrResourceNotFound")
	assert(t, len(files) == 0, "Expected zero files")
}

func TestMetaServiceShell(t *testing.T) {
	// Create temporary storage
	storage, err := runtime.NewTemporaryStorage(os.TempDir())
	if err != nil {
		panic("Failed to create TemporaryStorage")
	}

	// Setup a new MetaService
	log := bytes.NewBuffer(nil)
	meta := New([]string{"bash", "-c", "whoami"}, make(map[string]string), log, func(bool) {}, &runtime.Environment{
		TemporaryStorage: storage,
	})
	s := httptest.NewServer(meta)
	defer s.Close()

	debug("### Test shell running an echo service")

	go func() {
		// Start polling for an action
		req, err2 := http.NewRequest("GET", s.URL+"/engine/v1/poll", nil)
		nilOrFatal(t, err2)
		res, err2 := http.DefaultClient.Do(req)
		nilOrFatal(t, err2)
		assert(t, res.StatusCode == http.StatusOK)
		action := Action{}
		data, err2 := ioutil.ReadAll(res.Body)
		nilOrFatal(t, err2)
		err2 = json.Unmarshal(data, &action)
		nilOrFatal(t, err2, "Failed to decode JSON")

		// Check that the action is 'exec-shell' (as expected)
		assert(t, action.ID != "", "Expected action.ID != ''")
		assert(t, action.Type == "exec-shell", "Expected exec-shell action")
		assert(t, action.Path == "", "Didn't expect action.Path")

		dialer := websocket.Dialer{
			HandshakeTimeout: 30 * time.Second,
			ReadBufferSize:   8 * 1024,
			WriteBufferSize:  8 * 1024,
		}
		ws, _, err2 := dialer.Dial("ws:"+s.URL[5:]+"/engine/v1/reply?id="+action.ID, nil)
		nilOrFatal(t, err2, "Failed to open websocket")

		debug("guest-tool: Read: 'hi' on stdin")
		messageType, m, err2 := ws.ReadMessage()
		nilOrFatal(t, err2, "ReadMessage failed")
		assert(t, messageType == websocket.BinaryMessage, "expected BinaryMessage")
		assert(t, bytes.Equal(m, []byte{
			shellconsts.MessageTypeData, shellconsts.StreamStdin, 'h', 'i',
		}), "expected 'hi' on stdin")

		debug("guest-tool: Ack: 'hi' from stdin")
		err2 = ws.WriteMessage(websocket.BinaryMessage, []byte{
			shellconsts.MessageTypeAck, shellconsts.StreamStdin, 0, 0, 0, 2,
		})
		nilOrFatal(t, err2, "Failed to send ack")

		debug("guest-tool: Send: 'hello' on stdout")
		err2 = ws.WriteMessage(websocket.BinaryMessage, []byte{
			shellconsts.MessageTypeData, shellconsts.StreamStdout, 'h', 'e', 'l', 'l', 'o',
		})
		nilOrFatal(t, err2, "Failed to send 'hello'")

		debug("guest-tool: Read: ack for the 'hello'")
		messageType, m, err2 = ws.ReadMessage()
		nilOrFatal(t, err2, "Failed to ReadMessage")
		assert(t, messageType == websocket.BinaryMessage, "expected BinaryMessage")
		assert(t, bytes.Equal(m, []byte{
			shellconsts.MessageTypeAck, shellconsts.StreamStdout, 0, 0, 0, 5,
		}), "expected ack for 5 on stdout")

		debug("guest-tool: Send: close on stdout")
		err2 = ws.WriteMessage(websocket.BinaryMessage, []byte{
			shellconsts.MessageTypeData, shellconsts.StreamStdout,
		})
		nilOrFatal(t, err2, "Failed to send close for stdout")

		debug("guest-tool: Read: close for stdin")
		messageType, m, err2 = ws.ReadMessage()
		nilOrFatal(t, err2, "Failed to ReadMessage")
		assert(t, messageType == websocket.BinaryMessage, "expected BinaryMessage")
		assert(t, bytes.Equal(m, []byte{
			shellconsts.MessageTypeData, shellconsts.StreamStdin,
		}), "expected stdin to be closed")

		debug("guest-tool: Send: exit success")
		err2 = ws.WriteMessage(websocket.BinaryMessage, []byte{
			shellconsts.MessageTypeExit, 0,
		})
		nilOrFatal(t, err2, "Failed to send 'exit' success")
	}()

	// Exec shell through metaservice
	shell, err := meta.ExecShell(nil, false)
	assert(t, err == nil, "Unexpected error: ", err)

	debug("server: Writing 'hi' on stdin")
	_, err = shell.StdinPipe().Write([]byte("hi"))
	nilOrFatal(t, err, "Failed to write 'hi' on stdin")

	debug("server: Reading stdout (waiting for stdout to close)")
	b, err := ioutil.ReadAll(shell.StdoutPipe())
	nilOrFatal(t, err, "Failed to readAll from stdout")
	assert(t, string(b) == "hello", "Failed to read 'hello'")

	debug("server: Closing stdin")
	err = shell.StdinPipe().Close()
	nilOrFatal(t, err)

	debug("server: Waiting for exit success")
	success, err := shell.Wait()
	assert(t, err == nil, "Unexpected error: ", err)
	assert(t, success, "Expected success")

	debug("server: Reading nothing stderr (just check that it's closed)")
	b, err = ioutil.ReadAll(shell.StderrPipe())
	nilOrFatal(t, err, "Failed to readAll from stderr")
	assert(t, string(b) == "", "Failed to read ''")
}
