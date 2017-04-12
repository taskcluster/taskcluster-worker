// +build linux

package qemuguesttools

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/taskcluster/taskcluster-worker/engines/qemu/metaservice"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
	"github.com/taskcluster/taskcluster-worker/runtime/mocks"
)

func TestGuestToolsSuccess(t *testing.T) {
	// Create temporary storage
	storage, err := runtime.NewTemporaryStorage(os.TempDir())
	if err != nil {
		panic("Failed to create TemporaryStorage")
	}
	environment := &runtime.Environment{
		TemporaryStorage: storage,
		Monitor:          mocks.NewMockMonitor(true),
	}

	// Setup a new MetaService
	logTask := bytes.NewBuffer(nil)
	result := false
	var resolved atomics.Once
	s := metaservice.New([]string{"sh", "-c", "echo \"$TEST_TEXT\" && true"}, map[string]string{
		"TEST_TEXT": "Hello world",
	}, logTask, func(r bool) {
		if !resolved.Do(func() { result = r }) {
			panic("It shouldn't be possible to resolve twice")
		}
	}, environment)

	// Create http server for testing
	ts := httptest.NewServer(s)
	defer ts.Close()
	u, err := url.Parse(ts.URL)
	if err != nil {
		panic("Expected a url we can parse")
	}

	// Create an run guest-tools
	g := new(u.Host, mocks.NewMockMonitor(true))
	g.Run()

	// Check the state
	resolved.Wait()
	if !result {
		t.Error("Expected the metadata to get successful result")
	}
	if !strings.Contains(logTask.String(), "Hello world") {
		t.Error("Got unexpected taskLog: '", logTask.String(), "'")
	}
}

func TestGuestToolsFailed(t *testing.T) {
	// Create temporary storage
	storage, err := runtime.NewTemporaryStorage(os.TempDir())
	if err != nil {
		panic("Failed to create TemporaryStorage")
	}
	environment := &runtime.Environment{
		TemporaryStorage: storage,
		Monitor:          mocks.NewMockMonitor(true),
	}

	// Setup a new MetaService
	logTask := bytes.NewBuffer(nil)
	result := false
	var resolved atomics.Once
	s := metaservice.New([]string{"sh", "-c", "echo \"$TEST_TEXT\" && false"}, map[string]string{
		"TEST_TEXT": "Hello world",
	}, logTask, func(r bool) {
		if !resolved.Do(func() { result = r }) {
			panic("It shouldn't be possible to resolve twice")
		}
	}, environment)

	// Create http server for testing
	ts := httptest.NewServer(s)
	defer ts.Close()
	u, err := url.Parse(ts.URL)
	if err != nil {
		panic("Expected a url we can parse")
	}

	// Create an run guest-tools
	g := new(u.Host, mocks.NewMockMonitor(true))
	g.Run()

	// Check the state
	resolved.Wait()
	if result {
		t.Error("Expected the metadata to get failed result")
	}
	if !strings.Contains(logTask.String(), "Hello world") {
		t.Error("Got unexpected taskLog: '", logTask.String(), "'")
	}
}

func TestGuestToolsLiveLog(t *testing.T) {
	nowReady := sync.WaitGroup{}
	nowReady.Add(1)
	ps := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		debug("Waiting for ready-now to be readable in log")
		nowReady.Wait()
		debug("replying: request-ok")

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("request-ok"))
	}))

	// Create temporary storage
	storage, err := runtime.NewTemporaryStorage(os.TempDir())
	if err != nil {
		panic("Failed to create TemporaryStorage")
	}
	environment := &runtime.Environment{
		TemporaryStorage: storage,
	}

	// Setup a new MetaService
	reader, writer := io.Pipe()
	result := false
	var resolved atomics.Once
	s := metaservice.New([]string{"sh", "-c", "echo \"$TEST_TEXT\" && curl -s " + ps.URL}, map[string]string{
		"TEST_TEXT": "ready-now",
	}, writer, func(r bool) {
		if !resolved.Do(func() { result = r }) {
			panic("It shouldn't be possible to resolve twice")
		}
	}, environment)

	// Create http server for testing
	ts := httptest.NewServer(s)
	defer ts.Close()
	u, err := url.Parse(ts.URL)
	if err != nil {
		panic("Expected a url we can parse")
	}

	// Wait for
	logTask := bytes.NewBuffer(nil)
	logDone := sync.WaitGroup{}
	logDone.Add(1)
	go func() {
		b := make([]byte, 1)
		for !strings.Contains(logTask.String(), "ready-now") {
			n, err := reader.Read(b)
			logTask.Write(b[:n])
			if err != nil {
				panic("Unexpected error")
			}
		}
		nowReady.Done()
		io.Copy(logTask, reader)
		logDone.Done()
	}()

	// Create an run guest-tools
	g := new(u.Host, mocks.NewMockMonitor(true))
	g.Run()
	writer.Close()
	logDone.Wait()

	// Check the state
	resolved.Wait()
	if !result {
		t.Error("Expected the metadata to get successful result")
	}
	if !strings.Contains(logTask.String(), "request-ok") {
		t.Error("Got unexpected taskLog: '", logTask.String(), "'")
	}
}

func TestGuestToolsKill(t *testing.T) {
	// Create temporary storage
	storage, err := runtime.NewTemporaryStorage(os.TempDir())
	if err != nil {
		panic("Failed to create TemporaryStorage")
	}
	environment := &runtime.Environment{
		TemporaryStorage: storage,
		Monitor:          mocks.NewMockMonitor(true),
	}

	// Setup a new MetaService
	reader, writer := io.Pipe()
	result := false
	var resolved atomics.Once
	s := metaservice.New([]string{"sh", "-c", "echo \"$TEST_TEXT\" && sleep 20 && true"}, map[string]string{
		"TEST_TEXT": "kill-me-now",
	}, writer, func(r bool) {
		if !resolved.Do(func() { result = r }) {
			panic("It shouldn't be possible to resolve twice")
		}
	}, environment)

	// Create http server for testing
	ts := httptest.NewServer(s)
	defer ts.Close()
	defer s.StopPollers() // HACK: stop pollers or they will hang
	u, err := url.Parse(ts.URL)
	if err != nil {
		panic("Expected a url we can parse")
	}

	// Wait for
	logTask := bytes.NewBuffer(nil)
	logDone := sync.WaitGroup{}
	logDone.Add(1)
	go func() {
		b := make([]byte, 1)
		for !strings.Contains(logTask.String(), "kill-me-now") {
			n, err := reader.Read(b)
			logTask.Write(b[:n])
			if err != nil {
				panic("Unexpected error")
			}
		}
		debug("reached 'kill-me-now'")
		go func() {
			if err := s.KillProcess(); err != nil {
				panic(err)
			}
		}()
		io.Copy(logTask, reader)
		logDone.Done()
	}()

	// Create an run guest-tools
	g := new(u.Host, mocks.NewMockMonitor(true))

	// start processing actions
	go g.ProcessActions()
	defer g.StopProcessingActions()

	g.Run()
	writer.Close()
	logDone.Wait()

	// Check the state
	resolved.Wait()
	if result {
		t.Error("Expected the metadata to get failed result")
	}
}
