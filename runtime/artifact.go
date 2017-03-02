package runtime

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"time"

	got "github.com/taskcluster/go-got"
	"github.com/taskcluster/taskcluster-client-go"
	"github.com/taskcluster/taskcluster-client-go/queue"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

// S3Artifact wraps all of the needed fields to upload an s3 artifact
type S3Artifact struct {
	Name              string
	Mimetype          string
	Expires           tcclient.Time
	Stream            ioext.ReadSeekCloser
	AdditionalHeaders map[string]string
}

// ErrorArtifact wraps all of the needed fields to upload an error artifact
type ErrorArtifact struct {
	Name    string
	Message string
	Reason  string
	Expires tcclient.Time
}

// RedirectArtifact wraps all of the needed fields to upload a redirect artifact
type RedirectArtifact struct {
	Name     string
	Mimetype string
	URL      string
	Expires  tcclient.Time
}

// S3Artifacts returns a copy of all s3 artifact upload requests *made so far*
// for the given task context. Note it does not guarantee that the upload was
// successful. Typically this should be called during the Finished task phase,
// since plugins will typically upload artifacts in the Stopped task phase.
func (context *TaskContext) S3Artifacts() []queue.S3ArtifactRequest {
	context.artifactMutex.RLock()
	defer context.artifactMutex.Unlock()
	s3ArtifactsCopy := make([]queue.S3ArtifactRequest, len(context.s3Artifacts))
	copy(s3ArtifactsCopy, context.s3Artifacts)
	return s3ArtifactsCopy
}

// ErrorArtifacts returns a copy of all error artifact upload requests *made so
// far* for the given task context. Note it does not guarantee that the upload
// was successful. Typically this should be called during the Finished task
// phase, since plugins will typically upload artifacts in the Stopped task
// phase.
func (context *TaskContext) ErrorArtifacts() []queue.ErrorArtifactRequest {
	context.artifactMutex.RLock()
	defer context.artifactMutex.Unlock()
	errorArtifactsCopy := make([]queue.ErrorArtifactRequest, len(context.errorArtifacts))
	copy(errorArtifactsCopy, context.errorArtifacts)
	return errorArtifactsCopy
}

// RedirectArtifacts returns a copy of all redirect artifact upload requests
// *made so far* for the given task context. Note it does not guarantee that
// the upload was successful. Typically this should be called during the
// Finished task phase, since plugins will typically upload artifacts in the
// Stopped task phase.
func (context *TaskContext) RedirectArtifacts() []queue.RedirectArtifactRequest {
	context.artifactMutex.RLock()
	defer context.artifactMutex.Unlock()
	redirectArtifactsCopy := make([]queue.RedirectArtifactRequest, len(context.redirectArtifacts))
	copy(redirectArtifactsCopy, context.redirectArtifacts)
	return redirectArtifactsCopy
}

// UploadS3Artifact is responsible for creating new artifacts
// in the queue and then performing the upload to s3.
func (context *TaskContext) UploadS3Artifact(artifact S3Artifact) error {
	s3Artifact := queue.S3ArtifactRequest{
		ContentType: artifact.Mimetype,
		Expires:     artifact.Expires,
		StorageType: "s3",
	}
	context.artifactMutex.Lock()
	context.s3Artifacts = append(context.s3Artifacts, s3Artifact)
	context.artifactMutex.Unlock()

	req, err := json.Marshal(s3Artifact)
	if err != nil {
		return err
	}

	parsed, err := context.createArtifact(artifact.Name, req)
	if err != nil {
		return err
	}
	var resp queue.S3ArtifactResponse
	err = json.Unmarshal(parsed, &resp)
	if err != nil {
		return err
	}

	return putArtifact(resp.PutURL, artifact.Mimetype, artifact.Stream, artifact.AdditionalHeaders)
}

// CreateErrorArtifact is responsible for inserting error
// artifacts into the queue.
func (context *TaskContext) CreateErrorArtifact(artifact ErrorArtifact) error {
	errorArtifact := queue.ErrorArtifactRequest{
		Message:     artifact.Message,
		Reason:      artifact.Reason,
		Expires:     artifact.Expires,
		StorageType: "error",
	}
	context.artifactMutex.Lock()
	context.errorArtifacts = append(context.errorArtifacts, errorArtifact)
	context.artifactMutex.Unlock()

	req, err := json.Marshal(errorArtifact)
	if err != nil {
		return err
	}

	parsed, err := context.createArtifact(artifact.Name, req)
	if err != nil {
		return err
	}

	var resp queue.ErrorArtifactResponse
	return json.Unmarshal(parsed, &resp)
}

// CreateRedirectArtifact is responsible for inserting redirect
// artifacts into the queue.
func (context *TaskContext) CreateRedirectArtifact(artifact RedirectArtifact) error {
	redirectArtifact := queue.RedirectArtifactRequest{
		ContentType: artifact.Mimetype,
		URL:         artifact.URL,
		Expires:     artifact.Expires,
		StorageType: "reference",
	}
	context.artifactMutex.Lock()
	context.redirectArtifacts = append(context.redirectArtifacts, redirectArtifact)
	context.artifactMutex.Unlock()

	req, err := json.Marshal(redirectArtifact)
	if err != nil {
		return err
	}

	parsed, err := context.createArtifact(artifact.Name, req)
	if err != nil {
		return err
	}

	var resp queue.RedirectArtifactResponse
	return json.Unmarshal(parsed, &resp)
}

func (context *TaskContext) createArtifact(name string, req []byte) ([]byte, error) {
	par := queue.PostArtifactRequest(req)
	parsp, err := context.Queue().CreateArtifact(
		context.TaskID,
		strconv.Itoa(context.RunID),
		name,
		&par,
	)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(*parsp), nil
}

func putArtifact(urlStr, mime string, stream ioext.ReadSeekCloser, additionalArtifacts map[string]string) error {
	u, err := url.Parse(urlStr)
	if err != nil {
		return err
	}
	contentLength, err := stream.Seek(0, 2)
	if err != nil {
		return err
	}

	header := make(http.Header)
	header.Set("content-type", mime)

	for k, v := range additionalArtifacts {
		header.Set(k, v)
	}

	backoff := got.DefaultBackOff
	attempts := 0
	client := &http.Client{
		Timeout: 10 * time.Minute, // There should be _some_ timeout, this seems like a good starting value.
	}
	for {
		attempts++
		stream.Seek(0, 0)
		req := &http.Request{
			Method:        "PUT",
			URL:           u,
			Proto:         "HTTP/1.1",
			ProtoMajor:    1,
			ProtoMinor:    1,
			Header:        header,
			Body:          stream,
			ContentLength: contentLength,
		}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode/100 == 4 {
			httpErr, err := httputil.DumpResponse(resp, true)
			if err != nil {
				return err
			}
			return errors.New(string(httpErr))
		}
		if resp.StatusCode/100 == 5 {
			// TODO: Make this configurable
			if attempts < 10 {
				time.Sleep(backoff.Delay(attempts))
				continue
			} else {
				httpErr, err := httputil.DumpResponse(resp, true)
				if err != nil {
					return err
				}
				return errors.New(string(httpErr))
			}
		}
		// If we've made it here, the upload has succeeded
		return nil
	}
}
