package fakequeue

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type restError struct {
	StatusCode int    `json:"-"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

var invalidJSONPayloadError = restError{
	StatusCode: http.StatusBadRequest,
	Code:       "InvalidJSONPayload",
	Message:    "Payload must be valid JSON",
}

var resourceNotFoundError = restError{
	StatusCode: http.StatusNotFound,
	Code:       "ResourceNotFound",
	Message:    "Check that the taskId and runId is correct",
}

type redirectResponse struct {
	StatusCode int
	Location   string
}

type rawResponse struct {
	StatusCode      int
	ContentType     string
	ContentEncoding string
	Payload         []byte
}

func reply(w http.ResponseWriter, r *http.Request, result interface{}) {
	if rr, ok := result.(rawResponse); ok {
		w.Header().Set("Content-Type", rr.ContentType)
		w.Header().Set("Content-Encoding", rr.ContentEncoding)
		w.WriteHeader(rr.StatusCode)
		w.Write(rr.Payload)
	} else if rd, ok := result.(redirectResponse); ok {
		http.Redirect(w, r, rd.Location, rd.StatusCode)
	} else if e, ok := result.(restError); ok {
		w.WriteHeader(e.StatusCode)
		data, _ := json.MarshalIndent(e, "", "  ")
		w.Write(data)
	} else {
		w.WriteHeader(http.StatusOK)
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			panic(fmt.Sprintf("internal server error: can't serialize: %T, err: %s", result, err))
		}
		w.Write(data)
	}
}
