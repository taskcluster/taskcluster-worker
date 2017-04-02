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
	tcclient "github.com/taskcluster/taskcluster-client-go"
	"github.com/taskcluster/taskcluster-client-go/queue"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

// S3Artifact wraps all of the needed fields to upload an s3 artifact
type S3Artifact struct {
	Name              string
	Mimetype          string
	Expires           time.Time
	Stream            ioext.ReadSeekCloser
	AdditionalHeaders map[string]string
}

// ErrorArtifact wraps all of the needed fields to upload an error artifact
type ErrorArtifact struct {
	Name    string
	Message string
	Reason  string
	Expires time.Time
}

// RedirectArtifact wraps all of the needed fields to upload a redirect artifact
type RedirectArtifact struct {
	Name     string
	Mimetype string
	URL      string
	Expires  time.Time
}

// UploadS3Artifact is responsible for creating new artifacts
// in the queue and then performing the upload to s3.
func (context *TaskContext) UploadS3Artifact(artifact S3Artifact) error {
	req, err := json.Marshal(queue.S3ArtifactRequest{
		ContentType: artifact.Mimetype,
		Expires:     tcclient.Time(artifact.Expires),
		StorageType: "s3",
	})
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
	req, err := json.Marshal(queue.ErrorArtifactRequest{
		Message:     artifact.Message,
		Reason:      artifact.Reason,
		Expires:     tcclient.Time(artifact.Expires),
		StorageType: "error",
	})
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
	req, err := json.Marshal(queue.RedirectArtifactRequest{
		ContentType: artifact.Mimetype,
		URL:         artifact.URL,
		Expires:     tcclient.Time(artifact.Expires),
		StorageType: "reference",
	})
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
