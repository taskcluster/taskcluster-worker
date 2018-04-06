package workertest

import (
	"bytes"
	"compress/gzip"
	"io/ioutil"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	got "github.com/taskcluster/go-got"
	"github.com/taskcluster/taskcluster-client-go/tcqueue"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

func verifyAssertions(t *testing.T, title, taskID string, task Task, q *tcqueue.Queue) {
	result, err := q.Status(taskID)
	assert.NoError(t, err, "Failed to fetch task status for '%s'", title)
	if err != nil {
		return
	}
	runID := len(result.Status.Runs) - 1
	debug("task: '%s' was resolved: %s, reason: %s in runId: %d",
		title, result.Status.State, result.Status.Runs[runID].ReasonResolved, runID)

	// Validate status assertions if we have one
	if task.Status != nil {
		task.Status(t, q, result.Status)
	}

	// check task resolution (if not set to ignore it)
	if !task.IgnoreState {
		if task.Exception == runtime.ReasonNoException {
			if task.Success {
				assert.Equal(t, "completed", result.Status.State, "Expected task completed")
			} else {
				assert.Equal(t, "failed", result.Status.State, "Expected task failed")
			}
		} else {
			assert.Equal(t, "exception", result.Status.State, "Expected task exception")
			assert.Equal(t, task.Exception.String(), result.Status.Runs[runID].ReasonResolved,
				"Expected an exception with reason: %s", task.Exception.String())
		}
	}

	// Create list of artifacts
	var artifacts []Artifact
	var continuationToken string
	for {
		r, err := q.ListArtifacts(taskID, strconv.Itoa(runID), continuationToken, "")
		assert.NoError(t, err, "Failed to list artifacts")
		if err != nil {
			break
		}

		// Append artifacts
		for _, a := range r.Artifacts {
			debug("task: '%s' has artifact: %s", title, a.Name)
			artifacts = append(artifacts, Artifact{
				Name:        a.Name,
				ContentType: a.ContentType,
				StorageType: a.StorageType,
				Expires:     time.Time(a.Expires),
				Data:        nil,
			})
		}

		// Break, if there is no continuationToken
		continuationToken = r.ContinuationToken
		if continuationToken == "" {
			break
		}
	}

	g := got.New()
	util.Spawn(len(artifacts), func(j int) {
		// Skip anything not s3 or azure
		if artifacts[j].StorageType != "s3" && artifacts[j].StorageType != "azure" {
			return
		}
		// Build signed URL
		u, err := q.GetArtifact_SignedURL(taskID, strconv.Itoa(runID), artifacts[j].Name, 15*time.Minute)
		assert.NoError(t, err, "Failed to create signed artifact URL, for: %s", artifacts[j].Name)
		if err != nil {
			return
		}

		// Get artifact
		res, err := g.Get(u.String()).Send()
		if err != nil {
			assert.NoError(t, err, "Failed to get artifact, for: %s", artifacts[j].Name)
			return
		}
		artifacts[j].ContentEncoding = res.Header.Get("Content-Encoding")
		if artifacts[j].ContentEncoding == "gzip" {
			r, err := gzip.NewReader(bytes.NewReader(res.Body))
			assert.NoError(t, err, "Failed to create gzip decoder for: %s", artifacts[j].Name)
			artifacts[j].Data, err = ioutil.ReadAll(r)
			assert.NoError(t, err, "Failed to decode gzip for: %s", artifacts[j].Name)
			err = r.Close()
			assert.NoError(t, err, "Failed to close gzip decoder for: %s", artifacts[j].Name)
		} else {
			artifacts[j].Data = res.Body
		}
	})

	// Validate artifacts
	if task.Artifacts == nil {
		assert.True(t, len(artifacts) == 0 || task.AllowAdditional,
			"task produced artifacts, but no assertions given and AllowAdditional is false")
	} else {
		util.Spawn(len(artifacts), func(j int) {
			a := artifacts[j]
			assertion, ok := task.Artifacts[a.Name]
			assert.True(t, ok || task.AllowAdditional, "Task produced artifact %s not allowed", a.Name)
			if assertion != nil {
				assertion(t, a)
			}
		})
		for name := range task.Artifacts {
			found := false
			for _, a := range artifacts {
				found = found || a.Name == name
			}
			assert.True(t, found, "Task did not produce expected artifact: %s", name)
		}
	}
}
