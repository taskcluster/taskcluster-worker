package osxnative

import (
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
)

// white list of environment variables that must
// be passed to the child process
var environmentWhitelist = []string{
	"PATH",
	"HOME",
	"USER",
	"SHELL",
	"TMPDIR",
	"PWD",
	"EDITOR",
	"LANG",
	"LOGNAME",
	"TERM",
	"TERM_PROGRAM",
	"TASK_ID",
	"RUN_ID",
	"TASKCLUSTER_WORKER_TYPE",
	"TASKCLUSTER_INSTANCE_TYPE",
	"TASKCLUSTER_WORKER_GROUP",
	"TASKCLUSTER_PUBLIC_IP",
}

type stdoutLogWriter struct {
	context *runtime.TaskContext
}

func (w stdoutLogWriter) Write(p []byte) (int, error) {
	w.context.Log(string(p))
	return len(p), nil
}

type stderrLogWriter struct {
	context *runtime.TaskContext
}

func (w stderrLogWriter) Write(p []byte) (int, error) {
	w.context.LogError(string(p))
	return len(p), nil
}

type sandbox struct {
	context     *runtime.TaskContext
	taskPayload *payload
	env         []string
	aborted     bool
}

func newSandbox(context *runtime.TaskContext, taskPayload *payload, env []string) *sandbox {
	return &sandbox{
		context:     context,
		taskPayload: taskPayload,
		env:         env,
		aborted:     false,
	}
}

func downloadLink(link string) (string, error) {
	resp, err := http.Get(link)

	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	contentDisposition := resp.Header.Get("Content-Disposition")
	_, params, err := mime.ParseMediaType(contentDisposition)

	if err != nil {
		return "", err
	}

	filename := params["filename"]
	file, err := os.Create(filename)

	if err != nil {
		return "", err
	}

	defer file.Close()
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		os.Remove(filename)
		return "", err
	}

	return filename, nil
}

func (s *sandbox) WaitForResult() (engines.ResultSet, error) {
	if s.aborted {
		return nil, engines.ErrSandboxAborted
	}

	if s.taskPayload.Link != "" {
		filename, err := downloadLink(s.taskPayload.Link)

		if err != nil {
			s.context.LogError(err)
			return nil, engines.ErrNonFatalInternalError
		}

		defer os.Remove(filename)

		err = os.Chmod(filename, 0777)

		if err != nil {
			s.context.LogError(err)
			return nil, engines.ErrNonFatalInternalError
		}
	}

	env := make([]string, len(s.env), len(s.env)+len(environmentWhitelist))
	copy(env, s.env)

	// Use the host environment plus custom environment variables
	for _, e := range environmentWhitelist {
		value, exists := os.LookupEnv(e)
		if exists {
			env = append(env, e+"="+value)
		}
	}

	cmd := exec.Command(s.taskPayload.Command[0], s.taskPayload.Command[1:]...)
	cmd.Stdout = stdoutLogWriter{s.context}
	cmd.Stderr = stderrLogWriter{s.context}
	cmd.Env = env

	err := cmd.Run()

	if err != nil {
		s.context.LogError("Command \"", s.taskPayload.Command, "\" failed to run: ", err)
		switch err.(type) {
		case *exec.ExitError:
			return newResultSet(s.context, false), nil
		default:
			return nil, engines.ErrNonFatalInternalError
		}
	}

	return newResultSet(s.context, true), nil
}

func (s *sandbox) Abort() error {
	s.aborted = true
	return nil
}

func (*sandbox) NewShell() (engines.Shell, error) {
	return nil, engines.ErrFeatureNotSupported
}

func (*sandbox) ListDisplays() ([]engines.Display, error) {

	return nil, engines.ErrFeatureNotSupported
}

func (*sandbox) OpenDisplay(string) (io.ReadWriteCloser, error) {
	return nil, engines.ErrFeatureNotSupported
}
