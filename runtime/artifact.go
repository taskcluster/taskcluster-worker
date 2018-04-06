package runtime

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"time"

	"github.com/pkg/errors"
	got "github.com/taskcluster/go-got"
	tcclient "github.com/taskcluster/taskcluster-client-go"
	"github.com/taskcluster/taskcluster-client-go/tcqueue"
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
	req, err := json.Marshal(tcqueue.S3ArtifactRequest{
		ContentType: artifact.Mimetype,
		Expires:     tcclient.Time(artifact.Expires),
		StorageType: "s3",
	})
	if err != nil {
		panic(errors.Wrap(err, "failed to Marshal json that should have worked"))
	}

	parsed, err := context.createArtifact(artifact.Name, req)
	if err != nil {
		return err
	}
	var resp tcqueue.S3ArtifactResponse
	if err = json.Unmarshal(parsed, &resp); err != nil {
		panic(errors.Wrap(err, "failed to parse JSON that have been parsed before"))
	}

	return putArtifact(resp.PutURL, artifact.Mimetype, artifact.Stream, artifact.AdditionalHeaders)
}

// CreateErrorArtifact is responsible for inserting error
// artifacts into the queue.
func (context *TaskContext) CreateErrorArtifact(artifact ErrorArtifact) error {
	req, err := json.Marshal(tcqueue.ErrorArtifactRequest{
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

	var resp tcqueue.ErrorArtifactResponse
	return json.Unmarshal(parsed, &resp)
}

// CreateRedirectArtifact is responsible for inserting redirect
// artifacts into the queue.
func (context *TaskContext) CreateRedirectArtifact(artifact RedirectArtifact) error {
	req, err := json.Marshal(tcqueue.RedirectArtifactRequest{
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

	var resp tcqueue.RedirectArtifactResponse
	return json.Unmarshal(parsed, &resp)
}

func (context *TaskContext) createArtifact(name string, req []byte) ([]byte, error) {
	par := tcqueue.PostArtifactRequest(req)
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
		panic(errors.Wrap(err, "failed to parse URL"))
	}
	contentLength, err := stream.Seek(0, io.SeekEnd)
	if err != nil {
		return errors.Wrap(err, "failed to seek end of stream for content-length detection")
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
		_, err := stream.Seek(0, io.SeekStart)
		if err != nil {
			return errors.Wrap(err, "Failed to seek start before uploading stream")
		}
		req := &http.Request{
			Method:        "PUT",
			URL:           u,
			Proto:         "HTTP/1.1",
			ProtoMajor:    1,
			ProtoMinor:    1,
			Header:        header,
			ContentLength: contentLength,
			Body:          stream,
			GetBody: func() (io.ReadCloser, error) {
				// In case we have to follow any redirects, which shouldn't happen
				if _, serr := stream.Seek(0, io.SeekStart); serr != nil {
					return nil, errors.Wrap(serr, "failed to seek to start of stream")
				}
				return ioutil.NopCloser(stream), nil
			},
		}
		resp, err := client.Do(req)
		if err != nil {
			if attempts < 10 {
				time.Sleep(backoff.Delay(attempts))
				continue
			}
			return errors.Wrap(err, "failed send request")
		}
		defer resp.Body.Close()
		if resp.StatusCode/100 == 4 {
			httpErr, err := httputil.DumpResponse(resp, true)
			if err != nil {
				return errors.Errorf("HTTP status: %d, and error dumping response: %s", resp.StatusCode, err)
			}
			return errors.Errorf("HTTP status: %d, response: %s", resp.StatusCode, string(httpErr))
		}
		if resp.StatusCode/100 == 5 {
			// TODO: Make this configurable
			if attempts < 10 {
				time.Sleep(backoff.Delay(attempts))
				continue
			} else {
				httpErr, err := httputil.DumpResponse(resp, true)
				if err != nil {
					return errors.Errorf("HTTP status: %d, and error dumping response: %s", resp.StatusCode, err)
				}
				return errors.Errorf("HTTP status: %d, response: %s", resp.StatusCode, string(httpErr))
			}
		}
		// If we've made it here, the upload has succeeded
		return nil
	}
}
