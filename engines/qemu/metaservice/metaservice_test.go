package metaservice

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
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
	resolved := false
	s := New([]string{"bash", "-c", "whoami"}, make(map[string]string), log, func(r bool) {
		if resolved {
			panic("It shouldn't be possible to resolve twice")
		}
		resolved = true
		result = r
	}, &runtime.Environment{
		TemporaryStorage: storage,
	})

	// Upload some log
	req, err := http.NewRequest("POST", "http://169.254.169.254/engine/v1/log", bytes.NewBufferString("Hello World"))
	nilOrPanic(err)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	assert(w.Code == http.StatusOK)

	// Check the log
	if log.String() != "Hello World" {
		panic("Expected 'Hello World' in the log")
	}

	// Check that we can report success
	req, err = http.NewRequest("PUT", "http://169.254.169.254/engine/v1/success", nil)
	nilOrPanic(err)
	w = httptest.NewRecorder()
	s.ServeHTTP(w, req)
	assert(w.Code == http.StatusOK)

	// Check result
	assert(resolved)
	assert(result)

	// Check idempotency
	req, err = http.NewRequest("PUT", "http://169.254.169.254/engine/v1/success", nil)
	nilOrPanic(err)
	w = httptest.NewRecorder()
	s.ServeHTTP(w, req)
	assert(w.Code == http.StatusOK)

	// Check that we can have a conflict
	req, err = http.NewRequest("PUT", "http://169.254.169.254/engine/v1/failed", nil)
	nilOrPanic(err)
	w = httptest.NewRecorder()
	s.ServeHTTP(w, req)
	assert(w.Code == http.StatusConflict)

	debug("### Test polling and get-artifact")

	// Check that we can poll for an action, and reply with an artifact
	go func() {
		// Start polling for an action
		req, err := http.NewRequest("GET", "http://169.254.169.254/engine/v1/poll", nil)
		nilOrPanic(err)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, req)
		assert(w.Code == http.StatusOK)
		action := Action{}
		err = json.Unmarshal(w.Body.Bytes(), &action)
		nilOrPanic(err, "Failed to decode JSON")

		// Check that the action is 'get-artifact' (as expected)
		assert(action.ID != "", "Expected action.ID != ''")
		assert(action.Type == "get-artifact", "Expected get-artifact action")
		assert(action.Path == "/home/worker/test-file", "Expected action.Path")

		// Post back an artifact
		req, err = http.NewRequest("POST",
			"http://169.254.169.254/engine/v1/reply?id="+action.ID,
			bytes.NewBufferString("hello-world"),
		)
		nilOrPanic(err)
		w = httptest.NewRecorder()
		s.ServeHTTP(w, req)
		assert(w.Code == http.StatusOK)
	}()

	// Get artifact through metaservice
	f, err := s.GetArtifact("/home/worker/test-file")
	nilOrPanic(err, "Failed to get artifact")
	defer f.Close()
	b, err := ioutil.ReadAll(f)
	assert(string(b) == "hello-world", "Expected hello-world artifact")

	debug("### Test polling and get-artifact for non-existing file")

	// Check that we can poll for an action, and reply with an error not-found
	go func() {
		// Start polling for an action
		req, err := http.NewRequest("GET", "http://169.254.169.254/engine/v1/poll", nil)
		nilOrPanic(err)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, req)
		assert(w.Code == http.StatusOK)
		action := Action{}
		err = json.Unmarshal(w.Body.Bytes(), &action)
		nilOrPanic(err, "Failed to decode JSON")

		// Check that the action is 'get-artifact' (as expected)
		assert(action.ID != "", "Expected action.ID != ''")
		assert(action.Type == "get-artifact", "Expected get-artifact action")
		assert(action.Path == "/home/worker/wrong-file", "Expected action.Path")

		// Post back an artifact
		req, err = http.NewRequest("POST",
			"http://169.254.169.254/engine/v1/reply?id="+action.ID,
			nil,
		)
		nilOrPanic(err)
		req.Header.Set("X-Taskcluster-Worker-Error", "file-not-found")
		w = httptest.NewRecorder()
		s.ServeHTTP(w, req)
		assert(w.Code == http.StatusOK)
	}()

	// Get error for artifact through metaservice
	f, err = s.GetArtifact("/home/worker/wrong-file")
	assert(f == nil, "Didn't expect to get a file")
	assert(err == engines.ErrResourceNotFound, "Expected ErrResourceNotFound")

	debug("### Test polling and list-folder")

	// Check that we can poll for an action, and reply to a list-folder
	go func() {
		// Start polling for an action
		req, err := http.NewRequest("GET", "http://169.254.169.254/engine/v1/poll", nil)
		nilOrPanic(err)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, req)
		assert(w.Code == http.StatusOK)
		action := Action{}
		err = json.Unmarshal(w.Body.Bytes(), &action)
		nilOrPanic(err, "Failed to decode JSON")

		// Check that the action is 'list-folder' (as expected)
		assert(action.ID != "", "Expected action.ID != ''")
		assert(action.Type == "list-folder", "Expected list-folder action")
		assert(action.Path == "/home/worker/", "Expected action.Path")

		// Post back an reply
		payload, _ := json.Marshal(Files{
			Files:    []string{"/home/worker/test-file"},
			NotFound: false,
		})
		req, err = http.NewRequest("POST",
			"http://169.254.169.254/engine/v1/reply?id="+action.ID,
			bytes.NewBuffer(payload),
		)
		nilOrPanic(err)
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		s.ServeHTTP(w, req)
		assert(w.Code == http.StatusOK, "Unexpected status: ", w.Code)
	}()

	// List folder through metaservice
	files, err := s.ListFolder("/home/worker/")
	nilOrPanic(err, "Failed to list-folder")
	assert(len(files) == 1, "Expected one file")
	assert(files[0] == "/home/worker/test-file", "Got the wrong file")

	debug("### Test polling and list-folder (not-found)")

	// Check that we can poll for an action, and reply to a list-folder, not found
	go func() {
		// Start polling for an action
		req, err := http.NewRequest("GET", "http://169.254.169.254/engine/v1/poll", nil)
		nilOrPanic(err)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, req)
		assert(w.Code == http.StatusOK)
		action := Action{}
		err = json.Unmarshal(w.Body.Bytes(), &action)
		nilOrPanic(err, "Failed to decode JSON")

		// Check that the action is 'list-folder' (as expected)
		assert(action.ID != "", "Expected action.ID != ''")
		assert(action.Type == "list-folder", "Expected list-folder action")
		assert(action.Path == "/home/worker/missing/", "Expected action.Path")

		// Post back an reply
		payload, _ := json.Marshal(Files{
			Files:    nil,
			NotFound: true,
		})
		req, err = http.NewRequest("POST",
			"http://169.254.169.254/engine/v1/reply?id="+action.ID,
			bytes.NewBuffer(payload),
		)
		nilOrPanic(err)
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		s.ServeHTTP(w, req)
		assert(w.Code == http.StatusOK, "Unexpected status: ", w.Code)
	}()

	// List folder through metaservice
	files, err = s.ListFolder("/home/worker/missing/")
	assert(err == engines.ErrResourceNotFound, "Expected ErrResourceNotFound")
	assert(len(files) == 0, "Expected zero files")
}
