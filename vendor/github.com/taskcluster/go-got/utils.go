package got

import (
	"encoding/json"
	"fmt"
)

// Head returns a new HEAD request with settings from Got
func (g *Got) Head(url string) *Request {
	return g.NewRequest("HEAD", url, nil)
}

// Get returns a new GET request with settings from Got
func (g *Got) Get(url string) *Request {
	return g.NewRequest("GET", url, nil)
}

// Post returns a new POST request with settings from Got
func (g *Got) Post(url string, body []byte) *Request {
	return g.NewRequest("POST", url, body)
}

// Put returns a new PUT request with settings from Got
func (g *Got) Put(url string, body []byte) *Request {
	return g.NewRequest("PUT", url, body)
}

// Patch returns a new PATCH request with settings from Got
func (g *Got) Patch(url string, body []byte) *Request {
	return g.NewRequest("PATCH", url, body)
}

// Delete returns a new DELETE request with settings from Got
func (g *Got) Delete(url string) *Request {
	return g.NewRequest("DELETE", url, nil)
}

// JSON sets the body to a JSON object and sets Content-Type to application/json
func (r *Request) JSON(object interface{}) error {
	body, err := json.Marshal(object)
	if err == nil {
		r.Header.Set("Content-Type", "application/json")
		r.Body = body
	}
	return err
}

// String returns a string representation of the request for debugging
func (r *Request) String() string {
	return fmt.Sprint(
		"got.Request: ", r.Method, r.URL, " len(body) = ", len(r.Body),
	)
}

// String returns a string representation of the response for debugging
func (r *Response) String() string {
	return fmt.Sprint(
		"got.Response: ", r.StatusCode, " len(body) = ", len(r.Body),
	)
}
