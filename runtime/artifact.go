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

// TODO: Consider making Name, TaskID, RunID be in a BaseArtifact?
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
	MimeType string
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
	par := queue.PostArtifactRequest(req)
	parsp, _, err := context.Queue().CreateArtifact(
		context.TaskID,
		strconv.Itoa(context.RunID),
		artifact.Name,
		&par,
	)
	if err != nil {
		// TODO: Do something with all of these errors
	}
	var resp queue.S3ArtifactResponse
	err = json.Unmarshal(json.RawMessage(*parsp), &resp)
	if err != nil {
		// TODO: Do something with all of these errors
	}

	putArtifact(resp.PutURL, artifact.Mimetype, artifact.Stream)

	return err
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
	client := &http.Client{}
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

// CreateErrorArtifact is responsible for inserting error
// artifacts into the queue. TODO: More docs here.
func CreateErrorArtifact(artifact ErrorArtifact, context *TaskContext) error {
	fmt.Println(artifact.Name)
	return nil
}

// CreateRedirectArtifact is responsible for inserting redirect
// artifacts into the queue. TODO: More docs here.
func CreateRedirectArtifact(artifact RedirectArtifact, context *TaskContext) error {
	fmt.Println(artifact.Name)
	return nil
}
