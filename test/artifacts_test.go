// Package test includes all integration tests (typically that talk to Queue
// service). Unit tests go in package they are testing.
package test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/taskcluster/httpbackoff"
	tcclient "github.com/taskcluster/taskcluster-client-go"
	"github.com/taskcluster/taskcluster-client-go/queue"
	_ "github.com/taskcluster/taskcluster-worker/plugins/artifacts"
	_ "github.com/taskcluster/taskcluster-worker/plugins/livelog"
)

func validateArtifacts(t *testing.T, testName string, payloadArtifacts []PayloadArtifact, expected []Artifact) {

	expires := tcclient.Time(time.Now().Add(time.Minute * 30))
	for i := range payloadArtifacts {
		payloadArtifacts[i].Expires = expires
	}
	for i := range expected {
		expected[i].Expires = expires
	}

	payload := TaskPayload{
		Command:    copyArtifact("SampleArtifacts"),
		MaxRunTime: 30,
		Artifacts:  payloadArtifacts,
	}

	task, workerType := NewTestTask(testName)

	taskID, q := SubmitTask(t, task, payload)
	RunTestWorker(workerType, 1)
	resp, err := q.ListArtifacts(taskID, "0", "", "")
	if err != nil {
		t.Fatal("Error retrieving artifact metadata from queue")
	}

	// compare expected vs actual artifacts by converting artifacts to strings...
	a := make([]Artifact, len(resp.Artifacts))
	for i := range resp.Artifacts {
		a[i] = Artifact(resp.Artifacts[i])
	}
	if fmt.Sprintf("%#v", a) != fmt.Sprintf("%#v", expected) {
		t.Fatalf("Expected different artifacts to be generated...\nExpected:\n%#v\nActual:\n%#v", expected, a)
	}
}

// See the testdata/SampleArtifacts subdirectory of this project. This
// simulates adding it as a directory artifact in a task payload, and checks
// that all files underneath this directory are discovered and created as s3
// artifacts.
func TestDirectoryArtifacts(t *testing.T) {

	validateArtifacts(t, "TestDirectoryArtifacts",

		// what appears in task payload
		[]PayloadArtifact{{
			Name: "SampleArtifacts",
			Path: "SampleArtifacts",
			Type: "directory",
		}},

		// what we expect to discover on file system
		[]Artifact{
			{
				StorageType: "reference",
				Name:        "public/logs/live.log",
				ContentType: "text/plain",
			},
			{
				StorageType: "s3",
				Name:        "public/logs/live_backing.log",
				ContentType: "text/plain",
			},
			{
				StorageType: "s3",
				Name:        "SampleArtifacts/%%%/v/X",
				ContentType: "application/octet-stream",
			},
			{
				StorageType: "s3",
				Name:        "SampleArtifacts/_/X.txt",
				ContentType: "text/plain; charset=utf-8",
			},
			{
				StorageType: "s3",
				Name:        "SampleArtifacts/b/c/d.jpg",
				ContentType: "image/jpeg",
			},
		},
	)
}

// Task payload specifies a file artifact which doesn't exist on worker
func TestMissingFileArtifact(t *testing.T) {

	validateArtifacts(t, "TestMissingFileArtifact",

		// what appears in task payload
		[]PayloadArtifact{{
			Name: "TestMissingFileArtifact/no_such_file",
			Path: "TestMissingFileArtifact/no_such_file",
			Type: "file",
		}},

		// what we expect to discover on file system
		[]Artifact{
			{
				StorageType: "reference",
				Name:        "public/logs/live.log",
				ContentType: "text/plain",
			},
			{
				StorageType: "s3",
				Name:        "public/logs/live_backing.log",
				ContentType: "text/plain",
			},
			{
				StorageType: "error",
				Name:        "TestMissingFileArtifact/no_such_file",
				// Message:     "Could not read file '" + filepath.Join(taskContext.TaskDir, "TestMissingFileArtifact", "no_such_file") + "'",
				// Reason:      "file-missing-on-worker",
			},
		},
	)
}

// Task payload specifies a directory artifact which doesn't exist on worker
func TestMissingDirectoryArtifact(t *testing.T) {

	validateArtifacts(t, "TestMissingDirectoryArtifact",

		// what appears in task payload
		[]PayloadArtifact{{
			Name: "TestMissingDirectoryArtifact/no_such_dir",
			Path: "TestMissingDirectoryArtifact/no_such_dir",
			Type: "directory",
		}},

		// what we expect to discover on file system
		[]Artifact{
			{
				StorageType: "reference",
				Name:        "public/logs/live.log",
				ContentType: "text/plain",
			},
			{
				StorageType: "s3",
				Name:        "public/logs/live_backing.log",
				ContentType: "text/plain",
			},
			{
				StorageType: "error",
				Name:        "TestMissingDirectoryArtifact/no_such_dir",
				// Message: "Could not read directory '" + filepath.Join(taskContext.TaskDir, "TestMissingDirectoryArtifact", "no_such_dir") + "'",
				// Reason:  "file-missing-on-worker",
			},
		},
	)
}

// Task payload specifies a file artifact which is actually a directory on worker
func TestFileArtifactIsDirectory(t *testing.T) {

	validateArtifacts(t, "TestFileArtifactIsDirectory",

		// what appears in task payload
		[]PayloadArtifact{{
			Name: "SampleArtifacts/b/c",
			Path: "SampleArtifacts/b/c",
			Type: "file",
		}},

		// what we expect to discover on file system
		[]Artifact{
			{
				StorageType: "reference",
				Name:        "public/logs/live.log",
				ContentType: "text/plain",
			},
			{
				StorageType: "s3",
				Name:        "public/logs/live_backing.log",
				ContentType: "text/plain",
			},
			{
				StorageType: "error",
				Name:        "SampleArtifacts/b/c",
				// Message: "File artifact '" + filepath.Join(taskContext.TaskDir, "SampleArtifacts", "b", "c") + "' exists as a directory, not a file, on the worker",
				// Reason:  "invalid-resource-on-worker",
			},
		},
	)
}

// Task payload specifies a directory artifact which is a regular file on worker
func TestDirectoryArtifactIsFile(t *testing.T) {

	validateArtifacts(t, "TestDirectoryArtifactIsFile",

		// what appears in task payload
		[]PayloadArtifact{{
			Name: "SampleArtifacts/b/c/d.jpg",
			Path: "SampleArtifacts/b/c/d.jpg",
			Type: "directory",
		}},

		// what we expect to discover on file system
		[]Artifact{
			{
				StorageType: "reference",
				Name:        "public/logs/live.log",
				ContentType: "text/plain",
			},
			{
				StorageType: "s3",
				Name:        "public/logs/live_backing.log",
				ContentType: "text/plain",
			},
			{
				StorageType: "error",
				Name:        "SampleArtifacts/b/c/d.jpg",
				// Message: "Directory artifact '" + filepath.Join(taskContext.TaskDir, "SampleArtifacts", "b", "c", "d.jpg") + "' exists as a file, not a directory, on the worker",
				// Reason:  "invalid-resource-on-worker",
			},
		},
	)
}

func TestMissingArtifactFailsTest(t *testing.T) {

	expires := tcclient.Time(time.Now().Add(time.Minute * 30))

	payload := TaskPayload{
		Command:    helloGoodbye(),
		MaxRunTime: 30,
		Artifacts: []PayloadArtifact{
			{
				Expires: expires,
				Name:    "public/pretend/artifact",
				Path:    "Nonexistent/art i fact.txt",
				Type:    "file",
			},
		},
	}

	task, workerType := NewTestTask("TestMissingArtifactFailsTest")

	taskID, q := SubmitTask(t, task, payload)
	RunTestWorker(workerType, 1)
	status, err := q.Status(taskID)
	if err != nil {
		t.Fatal("Error retrieving status from queue")
	}
	if status.Status.State != "failed" {
		t.Fatalf("Expected state 'failed' but got state '%v'", status.Status.State)
	}
}

func TestUpload(t *testing.T) {

	expires := tcclient.Time(time.Now().Add(time.Minute * 30))

	task, workerType := NewTestTask("TestUpload")
	payload := TaskPayload{
		Command: copyArtifact("SampleArtifacts/_/X.txt"),
		Artifacts: []PayloadArtifact{
			{
				Expires: expires,
				Name:    "SampleArtifacts/_/X.txt",
				Path:    "SampleArtifacts/_/X.txt",
				Type:    "file",
			},
		},
		MaxRunTime: 30,
	}
	taskID, q := SubmitTask(t, task, payload)
	t.Logf("Task ID: %v", taskID)
	RunTestWorker(workerType, 1)

	// now check results
	// some required substrings - not all, just a selection
	expectedArtifacts := map[string]struct {
		extracts        []string
		contentEncoding string
		expires         tcclient.Time
	}{
		"public/logs/live_backing.log": {
			extracts: []string{
				"copying file(s)",
			},
			contentEncoding: "gzip",
			expires:         task.Expires,
		},
		"public/logs/live.log": {
			extracts: []string{
				"copying file",
			},
			contentEncoding: "gzip",
			expires:         task.Expires,
		},
		"SampleArtifacts/_/X.txt": {
			extracts: []string{
				"test artifact",
			},
			contentEncoding: "",
			expires:         payload.Artifacts[0].Expires,
		},
	}

	artifacts, err := q.ListArtifacts(taskID, "0", "", "")

	if err != nil {
		t.Fatalf("Error listing artifacts: %v", err)
	}

	actualArtifacts := make(map[string]struct {
		ContentType string        `json:"contentType"`
		Expires     tcclient.Time `json:"expires"`
		Name        string        `json:"name"`
		StorageType string        `json:"storageType"`
	}, len(artifacts.Artifacts))

	for _, actualArtifact := range artifacts.Artifacts {
		actualArtifacts[actualArtifact.Name] = actualArtifact
	}

	for artifact := range expectedArtifacts {
		if a, ok := actualArtifacts[artifact]; ok {
			if a.ContentType != "text/plain; charset=utf-8" {
				t.Errorf("Artifact %s should have mime type 'text/plain; charset=utf-8' but has '%s'", artifact, a.ContentType)
			}
			if a.Expires.String() != expectedArtifacts[artifact].expires.String() {
				t.Errorf("Artifact %s should have expiry '%s' but has '%s'", artifact, expires, a.Expires)
			}
		} else {
			t.Errorf("Artifact '%s' not created", artifact)
		}
	}

	// now check content was uploaded to Amazon, and is correct

	for artifact, content := range expectedArtifacts {
		var url *url.URL
		url, err = q.GetLatestArtifact_SignedURL(taskID, artifact, 10*time.Minute)
		if err != nil {
			t.Fatalf("Error trying to fetch artifacts from Amazon...\n%s", err)
		}
		// need to do this so Content-Encoding header isn't swallowed by Go for test later on
		tr := &http.Transport{
			DisableCompression: true,
		}
		client := &http.Client{Transport: tr}
		var rawResp *http.Response
		rawResp, _, err = httpbackoff.ClientGet(client, url.String())
		if err != nil {
			t.Fatalf("Error trying to fetch decompressed artifact from signed URL %s ...\n%s", url.String(), err)
		}
		var resp *http.Response
		resp, _, err = httpbackoff.Get(url.String())
		if err != nil {
			t.Fatalf("Error trying to fetch artifact from signed URL %s ...\n%s", url.String(), err)
		}
		var b []byte
		b, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Error trying to read response body of artifact from signed URL %s ...\n%s", url.String(), err)
		}
		for _, requiredSubstring := range content.extracts {
			if !strings.Contains(string(b), requiredSubstring) {
				t.Errorf("Artifact '%s': Could not find substring %q in '%s'", artifact, requiredSubstring, string(b))
			}
		}
		if actualContentEncoding := rawResp.Header.Get("Content-Encoding"); actualContentEncoding != content.contentEncoding {
			t.Fatalf("Expected Content-Encoding %q but got Content-Encoding %q for artifact %q from url %v", content.contentEncoding, actualContentEncoding, artifact, url)
		}
		if actualContentType := resp.Header.Get("Content-Type"); actualContentType != "text/plain; charset=utf-8" {
			t.Fatalf("Content-Type in signed URL %v response should be '%v' but is '%v'", url, "text/plain; charset=utf-8", actualContentType)
		}
	}

	var status *queue.TaskStatusResponse
	status, err = q.Status(taskID)
	if err != nil {
		t.Fatal("Error retrieving status from queue")
	}
	if status.Status.State != "completed" {
		t.Fatalf("Expected state 'completed' but got state '%v'", status.Status.State)
	}
}
