// +build darwin

package osxnative

import (
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
	osuser "os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

// white list of environment variables that must
// be passed to the child process
var environmentWhitelist = []string{
	"PATH",
	"HOME",
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
	engines.SandboxBase
	context     *runtime.TaskContext
	taskPayload *payload
	env         []string
	aborted     bool
	engine      *engine
}

func newSandbox(context *runtime.TaskContext, taskPayload *payload, env []string, engine *engine) *sandbox {
	return &sandbox{
		context:     context,
		taskPayload: taskPayload,
		env:         env,
		aborted:     false,
		engine:      engine,
	}
}

func downloadLink(destdir string, link string) (string, error) {
	resp, err := http.Get(link)

	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	contentDisposition := resp.Header.Get("Content-Disposition")
	_, params, err := mime.ParseMediaType(contentDisposition)

	var filename string
	if err == nil {
		filename = params["filename"]
	} else {
		filename = filepath.Base(link)
	}

	filename = filepath.Join(destdir, filename)
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

	var err error

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

	// USER and HOME are treated seperately because their values
	// depend on either we create the new user successfuly or not
	processUser := os.Getenv("USER")
	processHome := os.Getenv("HOME")

	// If we fail to create a new user, the most probable cause is that
	// we don't have enough permissions. Chances are that we are running
	// in in a development environment, so do not fail the task to tests
	// run successfully.
	u := user{}
	if err = u.create(); err != nil {
		s.context.LogError("Could not create user: ", err, "\n")
		exitError, ok := err.(*exec.ExitError)
		if ok {
			s.context.LogError(string(exitError.Stderr), "\n")
		}

		tcWorkerEnv, exists := os.LookupEnv("TASKCLUSTER_WORKER_ENV")

		if exists && strings.ToLower(tcWorkerEnv) == "production" {
			return nil, engines.ErrNonFatalInternalError
		}
	} else {
		defer func() {
			if err != nil {
				u.delete()
			}
		}()

		userInfo, err := osuser.Lookup(u.name)
		if err != nil {
			s.context.LogError("Error looking up for user \""+u.name+"\": ", err, "\n")
		} else {
			uid, err := strconv.ParseUint(userInfo.Uid, 10, 32)
			if err != nil {
				s.context.LogError("ParseUint failed to convert ", userInfo.Uid, ": ", err, "\n")
				return nil, engines.ErrNonFatalInternalError
			}

			gid, err := strconv.ParseUint(userInfo.Gid, 10, 32)
			if err != nil {
				s.context.LogError("ParseUint failed to convert ", userInfo.Gid, ": ", err, "\n")
				return nil, engines.ErrNonFatalInternalError
			}

			cmd.SysProcAttr = &syscall.SysProcAttr{
				Credential: &syscall.Credential{
					Uid:    uint32(uid),
					Gid:    uint32(gid),
					Groups: []uint32{},
				},
			}

			cmd.Dir = userInfo.HomeDir
			processUser = u.name
			processHome = userInfo.HomeDir
		}
	}

	env = append(env, "HOME="+processHome, "USER="+processUser)
	cmd.Env = env

	if s.taskPayload.Link != "" {
		filename, err := downloadLink(getWorkingDir(u, s.context), s.taskPayload.Link)

		if err != nil {
			s.context.LogError(err)
			return nil, engines.ErrNonFatalInternalError
		}

		defer os.Remove(filename)

		if err = os.Chmod(filename, 0777); err != nil {
			s.context.LogError(err, "\n")
			return nil, engines.ErrNonFatalInternalError
		}
	}

	r := resultset{
		ResultSetBase: engines.ResultSetBase{},
		taskUser:      u,
		context:       s.context,
		success:       false,
		engine:        s.engine,
	}

	if err = cmd.Run(); err != nil {
		s.context.LogError("Command \"", s.taskPayload.Command, "\" failed to run: ", err, "\n")
		switch err.(type) {
		case *exec.ExitError:
			err = nil // do not delete the user by the end of the function
			return r, nil
		default:
			return nil, engines.ErrNonFatalInternalError
		}
	}

	r.success = true
	return r, nil
}

func (s *sandbox) Abort() error {
	s.aborted = true
	return nil
}
