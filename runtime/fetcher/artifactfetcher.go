package fetcher

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	schematypes "github.com/taskcluster/go-schematypes"
)

// TODO: Change Fetcher s.t. we have a Fetcher.NewItem(...) constructor
//       and Item then has Scopes(), HashKey() and Fetch() methods.
//       That way we can handle artifacts referenced by index, or with
//       latest runId, if not supplied.

type artifactFetcher struct{}

// Artifact is a Fetcher for downloading from an taskId, artifact tuple
var Artifact Fetcher = artifactFetcher{}

var artifactSchema = schematypes.Object{
	Properties: schematypes.Properties{
		"taskId": schematypes.String{},
		"runId": schematypes.Integer{
			Minimum: 0,
			Maximum: 50,
		},
		"artifact": schematypes.String{},
	},
	Required: []string{"taskId", "runId", "artifact"},
}

type artifactReference struct {
	TaskID   string `json:"taskId"`
	RunID    int    `json:"runId"`
	Artifact string `json:"artifact"`
}

func (r *artifactReference) isPublic() bool {
	return strings.HasPrefix(r.Artifact, "public/")
}

func (artifactFetcher) Schema() schematypes.Schema {
	return artifactSchema
}

func (artifactFetcher) HashKey(ref interface{}) string {
	var r artifactReference
	schematypes.MustValidateAndMap(artifactSchema, ref, &r)
	return fmt.Sprintf("%s/%d/%s", r.TaskID, r.RunID, r.Artifact)
}

func (artifactFetcher) Scopes(ref interface{}) [][]string {
	var r artifactReference
	schematypes.MustValidateAndMap(artifactSchema, ref, &r)
	if r.isPublic() {
		return [][]string{{}} // Set containing the empty-scope-set
	}
	return [][]string{{"queue:get-artifact:" + r.Artifact}}
}

func (artifactFetcher) Fetch(ctx Context, ref interface{}, target WriteSeekReseter) error {
	var r artifactReference
	schematypes.MustValidateAndMap(artifactSchema, ref, &r)

	var u string
	if r.isPublic() {
		u = fmt.Sprintf("https://queue.taskcluster.net/v1/task/%s/runs/%d/artifacts/%s", r.TaskID, r.RunID, r.Artifact)
	} else {
		u2, err := ctx.Queue().GetArtifact_SignedURL(r.TaskID, strconv.Itoa(r.RunID), r.Artifact, 15*time.Minute)
		if err != nil {
			panic(errors.Wrap(err, "Client library shouldn't be able to fail here"))
		}
		u = u2.String()
	}

	return fetchURLWithRetries(ctx, u, target)
}
