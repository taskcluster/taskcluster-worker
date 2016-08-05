package got

import (
	"bytes"
	"errors"
	"net/http"
	"time"
)

// Logger is a simple loggging interface. This is implicitely implemented by
// the builtin log package as well as logrus.
type Logger interface {
	Println(v ...interface{})
}

// Got is a simple HTTP client
type Got struct {
	Client      *http.Client
	BackOff     *BackOff
	Retries     int
	IsTransient func(BadResponseCodeError) bool
	Log         Logger
	MaxSize     int64
	MakeRequest func(*Request) (*http.Request, error)
}

// A Request as is ready to be sent
type Request struct {
	Got
	Method string
	URL    string
	Header http.Header
	Body   []byte
}

// A Response as received
type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	Attempts   int
}

// DefaultClient is the default client used by got.New().
//
// Most importantly it has a sane timeout. Which is what you'll want for REST
// API calls.
var DefaultClient = &http.Client{
	Timeout: 30 * time.Second,
}

// New returns a new Got client with sane defaults for most small REST API
// requests.
//
// This includes:
//  - Max 5 retries,
//  - Exponential back-off: 100, 200, 400, 800 ms with 25% randomization
//  - Retry any 5xx error,
//  - ErrResponseTooLarge if response body is larger than 10 MiB,
//  - Timeout at 30 seconds,
//
// Notice, this different from the defaults you will get with a zero-value
// Got{} structure.
//
// Note: Depending on the API you are calling you may not wish to use retries,
// for verbs like POST, PATCH, etc. that aren't idemponent. In that case, you
// just set Got.Retries = 0, for the instance used for non-idemponent requests.
func New() *Got {
	return &Got{
		Client:      DefaultClient,
		BackOff:     DefaultBackOff,
		Retries:     5,
		IsTransient: DefaultIsTransient,
		Log:         nil,
		MaxSize:     10 * 1024 * 1024,
		MakeRequest: DefaultMakeRequest,
	}
}

// NewRequest returns a new request with settings from Got
func (g *Got) NewRequest(method string, url string, body []byte) *Request {
	return &Request{
		Got:    *g,
		Method: method,
		URL:    url,
		Header: make(http.Header), // always create a header, to avoiding nil errors
		Body:   body,
	}
}

// DefaultMakeRequest creates a http.Request from a got.Request.
//
// This is the default implementation of Got.MakeRequest, if nothing else is
// specified. MakeRequest can be specified, for special use-cases such as
// generating a new Authorization header for each retry.
func DefaultMakeRequest(r *Request) (*http.Request, error) {
	// Make a new http.Request
	req, err := http.NewRequest(r.Method, r.URL, bytes.NewReader(r.Body))
	if err != nil {
		return nil, err
	}

	// Clone headers
	for key, value := range r.Header {
		req.Header[key] = append([]string{}, value...)
	}

	return req, nil
}

// DefaultIsTransient determines if an error is transient or persistent.
//
// This is the default implementation of Got.IsTransient, if nothing else is
// specified. By implementing a custom variant of this method and specifying
// on Got, it is possible to decide which 5xx error to retry. Default
// implementation is to retry any 5xx error.
func DefaultIsTransient(response BadResponseCodeError) bool {
	return response.StatusCode/100 == 5
}

// Send will execute the HTTP request
func (r *Request) Send() (*Response, error) {
	// Sanity check to avoid people violating some sane REST restrictions
	// According to the spec a body may be present, but servers MUST ignore it.
	// It's safer to forbid the body completely to avoid bugs.
	if r.Body != nil && (r.Method == "HEAD" || r.Method == "GET" || r.Method == "DELETE") {
		return nil, errors.New("HEAD, GET and DELETE request should not carry a body")
	}

	// Get an http.Client (fallback to default)
	c := r.Client
	if c == nil {
		c = http.DefaultClient
	}

	// Get a MakeRequest method
	makeRequest := r.MakeRequest
	if makeRequest == nil {
		makeRequest = DefaultMakeRequest
	}

	// Get an isTransient function
	isTransient := r.IsTransient
	if isTransient == nil {
		isTransient = func(BadResponseCodeError) bool { return false }
	}

	// Get a backoff struct
	backoff := r.BackOff
	if backoff == nil {
		backoff = DefaultBackOff
	}

	attempts := 0
	for {
		attempts++

		// Declare variable up-front
		var resp *http.Response
		var body []byte
		var response Response

		// Create request
		req, err := makeRequest(r)
		if err != nil {
			return nil, err
		}

		// Do a request and ensure the body is closed
		resp, err = c.Do(req)
		if err != nil {
			if attempts <= r.Retries {
				goto retry
			}
			return nil, err
		}

		// Read the body
		body, err = readAtmost(resp.Body, r.MaxSize)
		if err != nil {
			if attempts <= r.Retries && err != ErrResponseTooLarge {
				goto retry
			}
		}

		// Create the response object
		response = Response{
			StatusCode: resp.StatusCode,
			Header:     resp.Header,
			Body:       body,
			Attempts:   attempts,
		}

		// Return an error for non-2xx status codes
		if response.StatusCode/100 != 2 {
			err := BadResponseCodeError{&response}
			if attempts <= r.Retries && isTransient(err) {
				goto retry
			}
			return nil, err
		}

		return &response, err
	retry:
		// Sleep and take another iteration of the loop
		delay := backoff.Delay(attempts)
		if r.Log != nil {
			r.Log.Println(
				"Retrying request for: '", r.URL, "' after error: '", "err",
				"' with delay: ", delay,
			)
		}
		time.Sleep(delay)
	}
}
