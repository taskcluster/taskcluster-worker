package metaservice

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMetaService(t *testing.T) {
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
}
