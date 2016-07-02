// +build linux

package qemuguesttools

import (
	"bytes"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/taskcluster/taskcluster-worker/engines/qemu/metaservice"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

func TestGuestToolsSuccess(t *testing.T) {
	// Setup a new MetaService
	logTask := bytes.NewBuffer(nil)
	result := false
	resolved := false
	m := sync.Mutex{}
	s := metaservice.New([]string{"sh", "-c", "echo \"$TEST_TEXT\" && true"}, map[string]string{
		"TEST_TEXT": "Hello world",
	}, logTask, func(r bool) {
		m.Lock()
		defer m.Unlock()
		if resolved {
			panic("It shouldn't be possible to resolve twice")
		}
		resolved = true
		result = r
	})

	// Create http server for testing
	ts := httptest.NewServer(s)
	defer ts.Close()
	u, err := url.Parse(ts.URL)
	if err != nil {
		panic("Expected a url we can parse")
	}

	// Create a logger
	logger, _ := runtime.CreateLogger("info")
	log := logger.WithField("component", "guest-tools-tests")

	// Create an run guest-tools
	g := new(u.Host, log)
	g.Run()

	// Check the state
	if !resolved {
		t.Error("Expected the metadata to have resolved the task")
	}
	if result != true {
		t.Error("Expected the metadata to get successful result")
	}
	if !strings.Contains(logTask.String(), "Hello world") {
		t.Error("Got unexpected taskLog: '", logTask.String(), "'")
	}
}

func TestGuestToolsFailed(t *testing.T) {
	// Setup a new MetaService
	logTask := bytes.NewBuffer(nil)
	result := false
	resolved := false
	m := sync.Mutex{}
	s := metaservice.New([]string{"sh", "-c", "echo \"$TEST_TEXT\" && false"}, map[string]string{
		"TEST_TEXT": "Hello world",
	}, logTask, func(r bool) {
		m.Lock()
		defer m.Unlock()
		if resolved {
			panic("It shouldn't be possible to resolve twice")
		}
		resolved = true
		result = r
	})

	// Create http server for testing
	ts := httptest.NewServer(s)
	defer ts.Close()
	u, err := url.Parse(ts.URL)
	if err != nil {
		panic("Expected a url we can parse")
	}

	// Create a logger
	logger, _ := runtime.CreateLogger("info")
	log := logger.WithField("component", "guest-tools-tests")

	// Create an run guest-tools
	g := new(u.Host, log)
	g.Run()

	// Check the state
	if !resolved {
		t.Error("Expected the metadata to have resolved the task")
	}
	if result != false {
		t.Error("Expected the metadata to get failed result")
	}
	if !strings.Contains(logTask.String(), "Hello world") {
		t.Error("Got unexpected taskLog: '", logTask.String(), "'")
	}
}
