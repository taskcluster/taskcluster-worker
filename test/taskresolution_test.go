package test

import (
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/taskcluster/httpbackoff"
)

// Test failure should resolve as "failed"
func TestFailureResolvesAsFailure(t *testing.T) {
	task, workerType := NewTestTask("TestFailureResolvesAsFailure")
	payload := TaskPayload{
		Command: failCommand(),
	}
	taskID, q := SubmitTask(t, task, payload)
	RunTestWorker(workerType)

	tsr, err := q.Status(taskID)
	if err != nil {
		t.Fatalf("Could not retrieve task status")
	}
	if tsr.Status.State != "failed" {
		t.Fatalf("Was expecting state %q but got %q", "failed", tsr.Status.State)
	}
}

func TestAbortAfterMaxRunTime(t *testing.T) {
	task, workerType := NewTestTask("TestAbortAfterMaxRunTime")
	payload := TaskPayload{
		Command:    sleep(4),
		MaxRunTime: 3,
	}
	taskID, q := SubmitTask(t, task, payload)
	RunTestWorker(workerType)

	tsr, err := q.Status(taskID)
	if err != nil {
		t.Fatalf("Could not retrieve task status")
	}
	if tsr.Status.State != "failed" {
		t.Fatalf("Was expecting state %q but got %q", "failed", tsr.Status.State)
	}
	// check uploaded log mentions abortion
	// note: we do this rather than local log, to check also log got uploaded
	// as failure path requires that task is resolved before logs are uploaded
	url, err := q.GetLatestArtifact_SignedURL(taskID, "public/logs/live_backing.log", 10*time.Minute)
	if err != nil {
		t.Fatalf("Cannot retrieve url for live_backing.log: %v", err)
	}
	resp, _, err := httpbackoff.Get(url.String())
	if err != nil {
		t.Fatalf("Could not download log: %v", err)
	}
	defer resp.Body.Close()
	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Error when trying to read log file over http: %v", err)
	}
	logtext := string(bytes)
	if !strings.Contains(logtext, "max run time exceeded") {
		t.Fatalf("Was expecting log file to mention task abortion, but it doesn't")
	}
}
