package osxnative

import (
	"io"
	"net/http"
	"net/http/httptest"
)

type httpServer struct {
	testServer *httptest.Server
	bodyMap    *map[string]string
}

type handler struct {
	bodyMap *map[string]string
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Path[1:]

	if filename == "" {
		w.WriteHeader(404)
		return
	}

	w.Header().Set("Content-Disposition", "attachment;filename="+filename)
	io.WriteString(w, (*h.bodyMap)[r.URL.Path])
}

func (s *httpServer) addHandle(path string, body string) {
	(*s.bodyMap)[path] = body
}

func (s *httpServer) close() {
	s.testServer.Close()
}

func (s *httpServer) url() string {
	return s.testServer.URL
}

func newHttpServer() *httpServer {
	m := make(map[string]string)
	return &httpServer{
		testServer: httptest.NewServer(handler{&m}),
		bodyMap:    &m,
	}
}
