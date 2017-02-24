package fetcher

import (
	"context"
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

// URL is Fetcher for downloading files from a URL.
var URL Fetcher = urlFetcher{}

var urlSchema schematypes.Schema = schematypes.URI{
	MetaData: schematypes.MetaData{
		Title:       "URL",
		Description: "URL to fetch resource from, this must be `http://` or `https://`.",
	},
}

func (urlFetcher) Schema() schematypes.Schema {
	return urlSchema
}

func (urlFetcher) HashKey(ref interface{}) string {
	var u string
	if schematypes.MustMap(urlSchema, ref, &u) != nil {
		panic(fmt.Sprintf("Reference: %#v doesn't satisfy Fetcher.Schema()", ref))
	}
	return u
}

func (urlFetcher) Scopes(ref interface{}) [][]string {
	if urlSchema.Validate(ref) != nil {
		panic(fmt.Sprintf("Reference: %#v doesn't satisfy Fetcher.Schema()", ref))
	}
	return [][]string{{}} // Set containing the empty-scope-set
}

func (urlFetcher) Fetch(ctx Context, ref interface{}, target WriteSeekReseter) error {
	var u string
	if schematypes.MustMap(urlSchema, ref, &u) != nil {
		panic(fmt.Sprintf("Reference: %#v doesn't satisfy Fetcher.Schema()", ref))
	}

	return fetchURLWithRetries(ctx, u, target)
}

func fetchURLWithRetries(ctx context.Context, u string, target WriteSeekReseter) error {
	retry := 0
	for {
		// Fetch URL, if no error then we're done
		err := fetchURL(ctx, u, target)
		if err == nil {
			return nil
		}

		// Otherwise, reset the target (if there was an error)
		target.Reset()

		// If err is a persistentError or retry greater than maxRetries
		// then we return an error
		retry++
		if _, ok := err.(persistentError); ok || retry > maxRetries {
			return fmt.Errorf("GET %s - %s", u, err)
		}

		// Sleep before we retry
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backOff.Delay(retry)):
		}
	}
}

// persistentError is used to wrap errors that shouldn't be retried
type persistentError string

func (e persistentError) Error() string {
	return string(e)
}

func newPersistentError(format string, a ...interface{}) error {
	return persistentError(fmt.Sprintf(format, a...))
}

func fetchURL(ctx context.Context, u string, target io.Writer) error {
	// Create a new request
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return newPersistentError("invalid URL: %s", err)
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
			return newPersistentError("status: %d, body: %s", res.StatusCode, body)
		}
		return fmt.Errorf("status: %d, body: %s", res.StatusCode, body)
	}

	// Otherwise copy body to target
	_, err = io.Copy(target, res.Body)
	if err != nil {
		return fmt.Errorf("connection broken: %s", err)
	}

	return nil
}
