// +build darwin

package osxnative

import (
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Sirupsen/logrus"
	assert "github.com/stretchr/testify/require"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
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

///

func newTestSandbox(taskPayload *payloadType, env []string) (*sandbox, error) {
	temp, err := runtime.NewTemporaryStorage(os.TempDir())
	if err != nil {
		return nil, err
	}

	context, _, err := runtime.NewTaskContext(temp.NewFilePath(), runtime.TaskInfo{}, nil)
	if err != nil {
		return nil, err
	}

	e := engine{
		EngineBase: engines.EngineBase{},
		log:        logrus.New().WithField("component", "test"),
	}

	return newSandbox(context, taskPayload, env, &e), nil
}

func TestWaitResult(t *testing.T) {
	testPayload := payloadType{
		Command: []string{"/bin/echo", "-n", "test"},
	}

	s, err := newTestSandbox(&testPayload, []string{})
	assert.NoError(t, err)

	r, err := s.WaitForResult()
	assert.NoError(t, err)

	defer r.Dispose()

	assert.True(t, r.Success())
}

func TestInvalidCommand(t *testing.T) {
	testPayload := payloadType{
		Command: []string{"Idontexist"},
	}

	s, err := newTestSandbox(&testPayload, []string{})
	assert.NoError(t, err)

	r, err := s.WaitForResult()
	assert.Equal(t, err, engines.ErrNonFatalInternalError)
	assert.Nil(t, r)
}

func TestFailedCommand(t *testing.T) {
	testPayload := payloadType{
		Command: []string{"/bin/ls", "/invalidpath"},
	}

	s, err := newTestSandbox(&testPayload, []string{})
	assert.NoError(t, err)

	r, err := s.WaitForResult()
	assert.NoError(t, err)

	defer r.Dispose()
	assert.False(t, r.Success())
}

func TestAbort(t *testing.T) {
	testPayload := payloadType{
		Command: []string{"/bin/echo", "-n", "test"},
	}

	s, err := newTestSandbox(&testPayload, []string{})
	assert.NoError(t, err)

	assert.NoError(t, s.Abort())

	_, err = s.WaitForResult()
	assert.Equal(t, err, engines.ErrSandboxAborted)
}

func TestDownloadLink(t *testing.T) {
	expectedContent := "test"

	s := newHTTPServer()
	s.addHandle("/test.txt", expectedContent)
	defer s.close()

	filename, err := downloadLink(".", s.url()+"/test.txt")
	assert.NoError(t, err)

	defer os.Remove(filename)

	data, err := ioutil.ReadFile(filename)
	assert.NoError(t, err)

	content := string(data)
	assert.Equal(t, content, expectedContent)
}

func TestExecDownloadedScript(t *testing.T) {
	serv := newHTTPServer()
	serv.addHandle("/test.sh", "#!/bin/sh\necho -n test\n")
	defer serv.close()

	testPayload := payloadType{
		Link:    serv.url() + "/test.sh",
		Command: []string{"./test.sh"},
	}

	s, err := newTestSandbox(&testPayload, []string{})
	assert.NoError(t, err)

	r, err := s.WaitForResult()
	assert.NoError(t, err)

	defer r.Dispose()
	assert.True(t, r.Success())
}
