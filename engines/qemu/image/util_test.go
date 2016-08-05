package image

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/taskcluster/slugid-go/slugid"
)

func fmtPanic(a ...interface{}) {
	panic(fmt.Sprintln(a...))
}

func nilOrPanic(err error, a ...interface{}) {
	if err != nil {
		fmtPanic(append(a, err)...)
	}
}

func assert(condition bool, a ...interface{}) {
	if !condition {
		fmtPanic(a...)
	}
}

func TestDownloadImageOK(t *testing.T) {
	// Setup a testserver we can test against
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello world"))
	}))
	defer s.Close()

	// Create temporary file
	targetFile := filepath.Join(os.TempDir(), slugid.Nice())
	defer os.Remove(targetFile)

	// Download test url to the target file
	err := DownloadImage(s.URL)(targetFile)
	nilOrPanic(err, "Failed to download from testserver")

	result, err := ioutil.ReadFile(targetFile)
	nilOrPanic(err, "Failed to read targetFile, error: ", err)
	text := string(result)
	assert(text == "hello world", "Expected hello world, got ", text)
}

func TestDownloadImageRetry(t *testing.T) {
	// Setup a testserver we can test against
	count := 0
	m := sync.Mutex{}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.Lock()
		defer m.Unlock()
		count++
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("hello world"))
	}))
	defer s.Close()

	// Create temporary file
	targetFile := filepath.Join(os.TempDir(), slugid.Nice())
	defer os.Remove(targetFile)

	// Download test url to the target file
	err := DownloadImage(s.URL)(targetFile)
	assert(err != nil, "Expected an error")
	assert(count == 7, "Expected 7 attempts, got: ", count)
}
