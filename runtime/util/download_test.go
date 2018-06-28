package util

import (
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	assert "github.com/stretchr/testify/require"
)

// Simple HTTP server for tests

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

func newHTTPServer() *httpServer {
	m := make(map[string]string)
	return &httpServer{
		testServer: httptest.NewServer(handler{&m}),
		bodyMap:    &m,
	}
}

func TestDownload(t *testing.T) {
	expectedContent := "test"

	s := newHTTPServer()
	s.addHandle("/test.txt", expectedContent)
	defer s.close()

	filename, err := Download(s.url()+"/test.txt", ".")
	assert.NoError(t, err)

	defer os.Remove(filename)

	data, err := ioutil.ReadFile(filename)
	assert.NoError(t, err)

	content := string(data)
	assert.Equal(t, content, expectedContent)
}

func TestDownloadError(t *testing.T) {
	s := newHTTPServer()
	defer s.close()

	_, err := Download(s.url()+"/", ".")
	assert.EqualError(t, err, "Got status: 404")
}
