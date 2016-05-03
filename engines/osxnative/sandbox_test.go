package osxnative

import (
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

	return newSandbox(context, taskPayload, env), nil
}

func TestWaitResult(t *testing.T) {
	testPayload := payload{
		Command: []string{"/bin/echo", "-n", "test"},
	}

	s, err := newTestSandbox(&testPayload, []string{})
	if err != nil {
		t.Fatal(err)
	}

	r, err := s.WaitForResult()

	if err != nil {
		t.Fatal(err)
	}

	if !r.Success() {
		t.Fatalf("Command was expected to be sucessful, but failed")
	}
}

func TestInvalidCommand(t *testing.T) {
	testPayload := payload{
		Command: []string{"Idontexist"},
	}

	s, err := newTestSandbox(&testPayload, []string{})
	if err != nil {
		t.Fatal(err)
	}

	r, err := s.WaitForResult()

	if err != engines.ErrNonFatalInternalError {
		t.Fatalf("WaitForResult should return ErrNonFatalInternalError, but it didn't")
	}

	if r != nil {
		t.Fatalf("ResultSet was expected to be nil")
	}
}

func TestFailedCommand(t *testing.T) {
	testPayload := payload{
		Command: []string{"/bin/ls", "/invalidpath"},
	}

	s, err := newTestSandbox(&testPayload, []string{})
	if err != nil {
		t.Fatal(err)
	}

	r, err := s.WaitForResult()

	if err != nil {
		t.Fatalf("WaitForResult should return nil but returned \"%v\"", err)
	}

	if r.Success() {
		t.Fatalf("Command was expected to fail, but succeed")
	}
}

func TestAbort(t *testing.T) {
	testPayload := payload{
		Command: []string{"/bin/echo", "-n", "test"},
	}

	s, err := newTestSandbox(&testPayload, []string{})
	if err != nil {
		t.Fatal(err)
	}

	err = s.Abort()
	if err != nil {
		t.Fatalf("Abort failed: %v", err)
	}

	_, err = s.WaitForResult()

	if err != engines.ErrSandboxAborted {
		t.Fatalf("WaitForResult should return ErrSandboxAborted, but returned %v", err)
	}
}

func TestDownloadLink(t *testing.T) {
	expectedContent := "test"

	s := newHttpServer()
	s.addHandle("/test.txt", expectedContent)
	defer s.close()

	filename, err := downloadLink(s.url() + "/test.txt")
	if err != nil {
		t.Fatal(err)
	}

	defer os.Remove(filename)

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if content != expectedContent {
		t.Fatalf(
			"File content expected to be \"%s\", but it contains \"%s\"",
			expectedContent,
			content,
		)
	}
}

func TestExecDownloadedScript(t *testing.T) {
	serv := newHttpServer()
	serv.addHandle("/test.sh", "#!/bin/sh\necho -n test\n")
	defer serv.close()

	testPayload := payload{
		Link:    serv.url() + "/test.sh",
		Command: []string{"./test.sh"},
	}

	s, err := newTestSandbox(&testPayload, []string{})
	if err != nil {
		t.Fatal(err)
	}

	r, err := s.WaitForResult()

	if err != nil {
		t.Fatal(err)
	}

	if !r.Success() {
		t.Fatalf("Command was expected to be sucessful, but failed")
	}
}
