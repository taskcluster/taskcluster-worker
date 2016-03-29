package runtime

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"

	got "github.com/taskcluster/go-got"
	"github.com/taskcluster/taskcluster-client-go/queue"
	"github.com/taskcluster/taskcluster-client-go/tcclient"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

type S3Artifact struct {
	Name     string
	Mimetype string
	Expires  tcclient.Time
	Stream   ioext.ReadSeekCloser
}

type ErrorArtifact struct {
	Name    string
	Message string
	Reason  string
	Expires tcclient.Time
}

type RedirectArtifact struct {
	Name     string
	Mimetype string
	URL      string
	Expires  tcclient.Time
}

// UploadS3Artifact is responsible for creating new artifacts
// in the queue and then performing the upload to s3.
// TODO: More docs here.
func UploadS3Artifact(artifact S3Artifact, context *TaskContext) error {
	req, err := json.Marshal(queue.S3ArtifactRequest{
		ContentType: artifact.Mimetype,
		Expires:     artifact.Expires,
		StorageType: "s3",
	})
	parsed, err := createArtifact(context, artifact.Name, req)
	if err != nil {
		// TODO: Do something CORRECT with all of these errors
		return err
	}
	var resp queue.S3ArtifactResponse
	err = json.Unmarshal(parsed, &resp)
	if err != nil {
		// TODO: Do something CORRECT with all of these errors
		return err
	}

	putArtifact(resp.PutURL, artifact.Mimetype, artifact.Stream)

	return err
}

// CreateErrorArtifact is responsible for inserting error
// artifacts into the queue. TODO: More docs here.
func CreateErrorArtifact(artifact ErrorArtifact, context *TaskContext) error {
	req, err := json.Marshal(queue.ErrorArtifactRequest{
		Message:     artifact.Message,
		Reason:      artifact.Reason,
		Expires:     artifact.Expires,
		StorageType: "error",
	})
	parsed, err := createArtifact(context, artifact.Name, req)
	var resp queue.ErrorArtifactResponse
	err = json.Unmarshal(parsed, &resp)
	if err != nil {
		// TODO: Do something with all of these errors
	}
	return nil
}

// CreateRedirectArtifact is responsible for inserting redirect
// artifacts into the queue. TODO: More docs here.
func CreateRedirectArtifact(artifact RedirectArtifact, context *TaskContext) error {
	req, err := json.Marshal(queue.RedirectArtifactRequest{
		ContentType: artifact.Mimetype,
		URL:         artifact.URL,
		Expires:     artifact.Expires,
		StorageType: "reference",
	})
	parsed, err := createArtifact(context, artifact.Name, req)
	var resp queue.RedirectArtifactResponse
	err = json.Unmarshal(parsed, &resp)
	if err != nil {
		// TODO: Do something with all of these errors
	}
	return nil
}

func createArtifact(context *TaskContext, name string, req []byte) ([]byte, error) {
	par := queue.PostArtifactRequest(req)
	parsp, _, err := context.Queue().CreateArtifact(
		context.TaskID,
		strconv.Itoa(context.RunID),
		name,
		&par,
	)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return json.RawMessage(*parsp), nil
}

func putArtifact(urlStr, mime string, stream ioext.ReadSeekCloser) error {
	// TODO: Make this do retries
	// TODO: Use https://golang.org/pkg/bufio/
	u, err := url.Parse(urlStr)
	contentLength, err := stream.Seek(0, 2)

	header := make(http.Header)
	header.Set("content-type", mime)

	// TODO: Mock out http client
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
		if err != nil || resp.StatusCode/100 != 2 {
			// TODO: Make this configurable
			if attempts < 5 {
				time.Sleep(backoff.Delay(attempts))
				continue
			}
		}
		rbody, err := ioutil.ReadAll(resp.Body)
		fmt.Println(string(rbody))
		break
	}
	return err
}
