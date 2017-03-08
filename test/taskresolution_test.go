package test

import "testing"

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
