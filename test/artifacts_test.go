// Package test includes all integration tests (typically that talk to Queue
// service). Unit tests go in package they are testing.
package test

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/taskcluster/httpbackoff"
	tcclient "github.com/taskcluster/taskcluster-client-go"
	"github.com/taskcluster/taskcluster-client-go/queue"
	_ "github.com/taskcluster/taskcluster-worker/plugins/artifacts"
	_ "github.com/taskcluster/taskcluster-worker/plugins/livelog"
)

func TestUpload(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Currently not supported on Windows")
	}

	expires := tcclient.Time(time.Now().Add(time.Minute * 30))

	task, workerType := NewTestTask("TestUpload")
	payload := TaskPayload{
		Command: helloGoodbye(),
		Artifacts: []struct {
			Expires tcclient.Time `json:"expires,omitempty"`
			Name    string        `json:"name"`
			Path    string        `json:"path"`
			Type    string        `json:"type"`
		}{
			{
				Expires: expires,
				Name:    "SampleArtifacts/_/X.txt",
				Path:    filepath.Join(testdata, "SampleArtifacts/_/X.txt"),
				Type:    "file",
			},
		},
	}
	taskID, q := SubmitTask(t, task, payload)
	t.Logf("Task ID: %v", taskID)
	RunTestWorker(workerType)

	// now check results
	// some required substrings - not all, just a selection
	expectedArtifacts := map[string]struct {
		extracts        []string
		contentEncoding string
		expires         tcclient.Time
	}{
		"public/logs/live_backing.log": {
			extracts: []string{
				"hello world!",
				"goodbye world!",
			},
			contentEncoding: "gzip",
			expires:         task.Expires,
		},
		"public/logs/live.log": {
			extracts: []string{
				"hello world!",
				"goodbye world!",
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
