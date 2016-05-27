// +build darwin

package osxnative

import (
	"github.com/Sirupsen/logrus"
	assert "github.com/stretchr/testify/require"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"io/ioutil"
	"os"
	"testing"
)

func newTestSandbox(taskPayload *payload, env []string) (*sandbox, error) {
	temp, err := runtime.NewTemporaryStorage(os.TempDir())
	if err != nil {
		return nil, err
	}

	context, _, err := runtime.NewTaskContext(temp.NewFilePath(), runtime.TaskInfo{})
	if err != nil {
		return nil, err
	}

	engine := engine{
		EngineBase: engines.EngineBase{},
		log:        logrus.New().WithField("component", "test"),
	}

	return newSandbox(context, taskPayload, env, &engine), nil
}

func TestWaitResult(t *testing.T) {
	testPayload := payload{
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
	testPayload := payload{
		Command: []string{"Idontexist"},
	}

	s, err := newTestSandbox(&testPayload, []string{})
	assert.NoError(t, err)

	r, err := s.WaitForResult()
	assert.Equal(t, err, engines.ErrNonFatalInternalError)
	assert.Nil(t, r)
}

func TestFailedCommand(t *testing.T) {
	testPayload := payload{
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
	testPayload := payload{
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

	testPayload := payload{
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
