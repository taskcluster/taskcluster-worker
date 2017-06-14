package fetcher

import (
	"fmt"
	"net/http"

	"github.com/pkg/errors"
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/httpbackoff"
	tcindex "github.com/taskcluster/taskcluster-client-go/index"
)

type indexFetcher struct{}

// Index is a Fetcher for downloading from an (index, artifact) tuple
var Index Fetcher = indexFetcher{}

var indexSchema = schematypes.Object{
	Properties: schematypes.Properties{
		"namespace": schematypes.String{},
		"artifact":  schematypes.String{},
	},
	Required: []string{"namespace", "artifact"},
}

type indexReference struct {
	Namespace string `json:"namespace"`
	Artifact  string `json:"artifact"`
}

func (indexFetcher) Schema() schematypes.Schema {
	return indexSchema
}

func (indexFetcher) NewReference(ctx Context, options interface{}) (Reference, error) {
	var r indexReference
	schematypes.MustValidateAndMap(indexSchema, options, &r)

	// Lookup index
	index := tcindex.New(nil)
	// TODO: Rewrite the golang taskcluster client library, so this isn't so ugly
	// TODO: Expose indexBaseUrl from TaskContext
	index.Context = ctx
	index.Authenticate = false
	ns, err := index.FindTask(r.Namespace)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if e, ok := err.(httpbackoff.BadHttpResponseCode); ok && e.HttpResponseCode == http.StatusNotFound {
			return nil, newBrokenReferenceError(fmt.Sprintf("taskId for namespace %s", r.Namespace), "no such namespace")
		}
		return nil, errors.Wrap(err, "failed to fetch taskId for namespace")
	}

	// Determine latest runId
	result, err := ctx.Queue().Status(ns.TaskID)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if e, ok := err.(httpbackoff.BadHttpResponseCode); ok && e.HttpResponseCode == http.StatusNotFound {
			return nil, newBrokenReferenceError(fmt.Sprintf("task status from %s", ns.TaskID), "no such task")
		}
		return nil, errors.Wrap(err, "failed to fetch task status")
	}

	// Note: By resolving the index namespace and determining the runId before we
	// return a reference the HashKey will be <taskId>/<runId>/<artifact>, thus,
	// we ensure things stored in caches based on HashKey don't get duplicated
	// because they are referenced by different names. Also ensures that new
	// resources are fetched if index changes between tasks.
	return &artifactReference{
		TaskID:   ns.TaskID,
		RunID:    len(result.Status.Runs) - 1,
		Artifact: r.Artifact,
	}, nil
}
