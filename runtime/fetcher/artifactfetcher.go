package fetcher

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/httpbackoff"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

type artifactFetcher struct{}

// Artifact is a Fetcher for downloading from an (taskId, artifact) tuple
var Artifact Fetcher = artifactFetcher{}

var artifactSchema = schematypes.Object{
	Title: "Artifact Reference",
	Description: util.Markdown(`
		Object referencing an artifact by name from a specific 'taskId' and optional 'runId'.
	`),
	Properties: schematypes.Properties{
		"taskId": schematypes.String{
			Title:       "TaskId",
			Description: util.Markdown(`'taskId' of task to fetch artifact from`),
			Pattern:     `^[A-Za-z0-9_-]{8}[Q-T][A-Za-z0-9_-][CGKOSWaeimquy26-][A-Za-z0-9_-]{10}[AQgw]$`,
		},
		"runId": schematypes.Integer{
			Title:       "RunId",
			Description: util.Markdown(`'runId' to fetch artifact from, defaults to latest 'runId' if omitted`),
			Minimum:     0,
			Maximum:     50,
		},
		"artifact": schematypes.String{
			Title:         "Artifact",
			Description:   util.Markdown(`Name of artifact to fetch.`),
			MaximumLength: 1024,
		},
	},
	Required: []string{"taskId", "artifact"},
}

type artifactReference struct {
	TaskID   string `json:"taskId"`
	RunID    int    `json:"runId"`
	Artifact string `json:"artifact"`
}

func (artifactFetcher) Schema() schematypes.Schema {
	return artifactSchema
}

func (artifactFetcher) NewReference(ctx Context, options interface{}) (Reference, error) {
	var r artifactReference
	r.RunID = -1 // if not given we'll want to detect latest runId
	schematypes.MustValidateAndMap(artifactSchema, options, &r)

	// Determine latest runId
	if r.RunID == -1 {
		result, err := ctx.Queue().Status(r.TaskID)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if e, ok := err.(httpbackoff.BadHttpResponseCode); ok && e.HttpResponseCode == http.StatusNotFound {
				return nil, newBrokenReferenceError(fmt.Sprintf("task status from %s", r.TaskID), "no such task")
			}
			return nil, errors.Wrap(err, "failed to fetch task status")
		}
		r.RunID = len(result.Status.Runs) - 1
	}
	return &r, nil
}

func (r *artifactReference) isPublic() bool {
	return strings.HasPrefix(r.Artifact, "public/")
}

func (r *artifactReference) HashKey() string {
	return fmt.Sprintf("%s/%d/%s", r.TaskID, r.RunID, r.Artifact)
}

func (r *artifactReference) Scopes() [][]string {
	if r.isPublic() {
		return [][]string{{}} // Set containing the empty-scope-set
	}
	return [][]string{{"queue:get-artifact:" + r.Artifact}}
}

func (r *artifactReference) Fetch(ctx Context, target WriteReseter) error {
	// Construct URL
	var u string
	if r.isPublic() {
		u = fmt.Sprintf("%s/v1/task/%s/runs/%d/artifacts/%s", runtime.GetServiceURL(ctx.RootURL(), "queue"), r.TaskID, r.RunID, r.Artifact)
	} else {
		u2, err := ctx.Queue().GetArtifact_SignedURL(r.TaskID, strconv.Itoa(r.RunID), r.Artifact, 25*time.Minute)
		if err != nil {
			panic(errors.Wrap(err, "Client library shouldn't be able to fail here"))
		}
		u = u2.String()
	}

	subject := fmt.Sprintf("artifact %s from %s/%d", r.Artifact, r.TaskID, r.RunID)
	return fetchURLWithRetries(ctx, subject, u, target)
}
