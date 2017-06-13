package fetcher

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
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

	// Note: We could have reused logic from urlfetcher.go, but then we would risk
	// leaking signed URLs into error messages.
	return fetchArtifactWithRetries(ctx, r, target)
}

func fetchArtifactWithRetries(ctx Context, r artifactReference, target WriteSeekReseter) error {
	retry := 0
	for {
		// Fetch artifact, if no error then we're done
		err := fetchArtifact(ctx, r, target)
		if err == nil {
			return nil
		}

		// Otherwise, reset the target (if there was an error)
		target.Reset()

		// If err is a persistentError or retry greater than maxRetries
		// then we return an error
		retry++
		if IsBrokenReferenceError(err) {
			return err
		}
		if retry > maxRetries {
			return newBrokenReferenceError(
				"exhausted retries trying to get artifact %s from %s/%d, last error: %s",
				r.Artifact, r.TaskID, r.RunID, err)
		}

		// Sleep before we retry
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backOff.Delay(retry)):
		}
	}
}

func fetchArtifact(ctx Context, r artifactReference, target io.Writer) error {
	// Construct URL
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

	// Create a new request
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		panic(errors.Wrap(err, "invalid URL shoudln't be possible"))
	}

	// Do the request with context
	req = req.WithContext(ctx)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %s", err)
	}
	defer res.Body.Close()

	// If status code isn't 200, we return an error
	if res.StatusCode != http.StatusOK {
		// Attempt to read body from request
		var body string
		if res.Body != nil {
			p, _ := ioext.ReadAtMost(res.Body, 8*1024) // limit to 8 kb
			body = string(p)
		}
		if 400 <= res.StatusCode && res.StatusCode < 500 {
			return newBrokenReferenceError(
				"failed to fetch artifact %s from %s/%d, statusCode: %d, body: %s",
				r.Artifact, r.TaskID, r.RunID, res.StatusCode, body)
		}
		return fmt.Errorf("statusCode: %d, body: %s", res.StatusCode, body)
	}

	// Otherwise copy body to target
	_, err = io.Copy(target, res.Body)
	if err != nil {
		return fmt.Errorf("connection broken: %s", err)
	}

	return nil
}
