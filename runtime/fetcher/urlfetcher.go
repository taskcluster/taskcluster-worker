package fetcher

import (
	"fmt"
	"io"
	"net/http"
	"time"

	got "github.com/taskcluster/go-got"
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

// Maximum number of retries when fetching a URL
const maxRetries = 8

// Backoff strategy for retrying requests
var backOff = got.BackOff{
	DelayFactor:         200 * time.Millisecond,
	RandomizationFactor: 0.25,
	MaxDelay:            60 * time.Second,
}

type urlFetcher struct{}

type urlReference struct {
	url string
}

// URL is Fetcher for downloading files from a URL.
var URL Fetcher = urlFetcher{}

var urlSchema schematypes.Schema = schematypes.URI{
	Title:       "URL",
	Description: "URL to fetch resource from, this must be `http://` or `https://`.",
}

func (urlFetcher) Schema() schematypes.Schema {
	return urlSchema
}

func (urlFetcher) NewReference(ctx Context, options interface{}) (Reference, error) {
	var u string
	schematypes.MustValidateAndMap(urlSchema, options, &u)
	return &urlReference{url: u}, nil
}

func (u *urlReference) HashKey() string {
	return u.url
}

func (u *urlReference) Scopes() [][]string {
	return [][]string{{}} // Set containing the empty-scope-set
}

func (u *urlReference) Fetch(ctx Context, target WriteSeekReseter) error {
	return fetchURLWithRetries(ctx, u.url, u.url, target)
}

// fetchURLWithRetries will download URL u to target with retries, using subject
// in error messages and progress updates
func fetchURLWithRetries(ctx Context, subject, u string, target WriteSeekReseter) error {
	retry := 0
	for {
		// Fetch URL, if no error then we're done
		err := fetchURL(ctx, subject, u, target)
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
			return newBrokenReferenceError(subject, fmt.Sprintf("exhausted retries with last error: %s", err))
		}

		// Sleep before we retry
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backOff.Delay(retry)):
		}
	}
}

func fetchURL(ctx Context, subject, u string, target io.Writer) error {
	// Create a new request
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return newBrokenReferenceError(subject, "invalid URL")
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
			return newBrokenReferenceError(subject, fmt.Sprintf("statusCode: %d, body: %s", res.StatusCode, body))
		}
		return fmt.Errorf("statusCode: %d, body: %s", res.StatusCode, body)
	}

	// Report download progress
	r := ioext.TellReader{Reader: res.Body}
	// We only progress, if some content length is provided
	done := make(chan struct{})
	finishedReporting := make(chan struct{})
	if res.ContentLength != -1 {
		ctx.Progress(subject, 0)
		go func() {
			defer close(finishedReporting)
			for {
				select {
				case <-time.After(progressReportInterval):
					ctx.Progress(subject, float64(r.Tell())/float64(res.ContentLength))
				case <-ctx.Done():
					return
				case <-done:
					return
				}
			}
		}()
	} else {
		close(finishedReporting)
	}

	// Copy body to target
	_, err = io.Copy(target, &r)

	close(done)         // Stop progress reporting
	<-finishedReporting // wait for reporting to be finished

	// Return any error
	if err != nil {
		return fmt.Errorf("connection broken: %s", err)
	}

	// Report download completed
	if res.ContentLength != -1 {
		ctx.Progress(subject, 1)
	}

	return nil
}
