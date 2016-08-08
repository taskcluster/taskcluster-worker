package qemubuild

import (
	"io"
	"net/http"
)

// logService is a minimalistic implementation of metadata service that allows
// for streaming out logs. This is useful for when we do automatic image builds.
type logService struct {
	Destination io.Writer
}

func (l *logService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != "/engine/v1/log" {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	defer r.Body.Close()
	_, err := io.Copy(l.Destination, r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
