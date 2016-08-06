package qemubuild

import (
	"io"
	"net/http"
)

type logService struct {
	Destination io.Writer
}

func (l *logService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != "/v1/log" {
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
