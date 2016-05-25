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
	"github.com/taskcluster/taskcluster-client-go/queue"
	"github.com/taskcluster/taskcluster-client-go/tcclient"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

// S3Artifact wraps all of the needed fields to upload an s3 artifact
type S3Artifact struct {
	Name     string
	Mimetype string
	Expires  tcclient.Time
	Stream   ioext.ReadSeekCloser
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

// UploadS3Artifact is responsible for creating new artifacts
// in the queue and then performing the upload to s3.
func UploadS3Artifact(artifact S3Artifact, context *TaskContext) error {
	req, err := json.Marshal(queue.S3ArtifactRequest{
		ContentType: artifact.Mimetype,
		Expires:     artifact.Expires,
		StorageType: "s3",
	})
	if err != nil {
		return err
	}

	parsed, err := createArtifact(context, artifact.Name, req)
	if err != nil {
		return err
	}
	var resp queue.S3ArtifactResponse
	err = json.Unmarshal(parsed, &resp)
	if err != nil {
		return err
	}

	err = putArtifact(resp.PutURL, artifact.Mimetype, artifact.Stream)
	if err != nil {
		return err
	}

	return nil
}

// CreateErrorArtifact is responsible for inserting error
// artifacts into the queue.
func CreateErrorArtifact(artifact ErrorArtifact, context *TaskContext) error {
	req, err := json.Marshal(queue.ErrorArtifactRequest{
		Message:     artifact.Message,
		Reason:      artifact.Reason,
		Expires:     artifact.Expires,
		StorageType: "error",
	})
	if err != nil {
		return err
	}

	parsed, err := createArtifact(context, artifact.Name, req)
	if err != nil {
		return err
	}

	var resp queue.ErrorArtifactResponse
	err = json.Unmarshal(parsed, &resp)
	if err != nil {
		return err
	}
	return nil
}

// CreateRedirectArtifact is responsible for inserting redirect
// artifacts into the queue.
func CreateRedirectArtifact(artifact RedirectArtifact, context *TaskContext) error {
	req, err := json.Marshal(queue.RedirectArtifactRequest{
		ContentType: artifact.Mimetype,
		URL:         artifact.URL,
		Expires:     artifact.Expires,
		StorageType: "reference",
	})
	if err != nil {
		return err
	}

	parsed, err := createArtifact(context, artifact.Name, req)
	if err != nil {
		return err
	}

	var resp queue.RedirectArtifactResponse
	err = json.Unmarshal(parsed, &resp)
	if err != nil {
		return err
	}

	return nil
}

func createArtifact(context *TaskContext, name string, req []byte) ([]byte, error) {
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

func putArtifact(urlStr, mime string, stream ioext.ReadSeekCloser) error {
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
