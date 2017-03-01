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

type sandbox struct {
	engines.SandboxBase
	context     *runtime.TaskContext
	taskPayload *payloadType
	env         []string
	aborted     bool
	engine      *engine
	monitor     runtime.Monitor
}

func newSandbox(context *runtime.TaskContext, taskPayload *payloadType, env []string, engine *engine) *sandbox {
	return &sandbox{
		context:     context,
		taskPayload: taskPayload,
		env:         env,
		aborted:     false,
		engine:      engine,
		monitor:     engine.monitor,
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
	cmd.Stdout = s.context.LogDrain()
	cmd.Stderr = s.context.LogDrain()

	userInfo, err := osuser.Current()
	if err != nil {
		s.monitor.Error("Error getting user information: ", err)
		return nil, engines.NewInternalError(err.Error())
	}

	u := newUser(s.engine.config.Sudo)

	if s.engine.config.CreateUser {
		if err = u.create(s.engine.config.UserGroups); err != nil {
			s.monitor.Error("Could not create user", err)
			exitError, ok := err.(*exec.ExitError)
			if ok {
				s.monitor.Error(string(exitError.Stderr))
			}

			return nil, engines.NewInternalError("Could not create temporary user")
		}

		defer func() {
			if err != nil {
				u.delete()
			}
		}()

		userInfo, err = osuser.Lookup(u.name)
		if err != nil {
			s.monitor.Error("Error looking up for user \""+u.name+"\"", err)
			return nil, engines.NewInternalError(err.Error())
		}

		var uid uint64
		uid, err = strconv.ParseUint(userInfo.Uid, 10, 32)
		if err != nil {
			s.monitor.Error("ParseUint failed to convert ", userInfo.Uid, err)
			return nil, engines.ErrNonFatalInternalError
		}

		var gid uint64
		gid, err = strconv.ParseUint(userInfo.Gid, 10, 32)
		if err != nil {
			s.monitor.Error("ParseUint failed to convert ", userInfo.Gid, err)
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
	}

	env = append(env, "HOME="+userInfo.HomeDir, "USER="+userInfo.Name)
	cmd.Env = env

	r := resultset{
		ResultSetBase: engines.ResultSetBase{},
		taskUser:      u,
		context:       s.context,
		success:       false,
		engine:        s.engine,
	}

	if s.taskPayload.Link != "" {
		var filename string
		filename, err = downloadLink(getWorkingDir(u, s.context), s.taskPayload.Link)

		if err != nil {
			s.context.LogError("Could not download ", s.taskPayload.Link, ": ", err)
			return r, nil
		}

		defer os.Remove(filename)

		if err = os.Chmod(filename, 0777); err != nil {
			s.monitor.Error("Could not set permissions in the file", err)
			return nil, engines.ErrNonFatalInternalError
		}
	}

	if err = cmd.Run(); err != nil {
		s.context.LogError("Command \"", s.taskPayload.Command, "\" failed to run: ", err)
		err = nil // do not delete the user by the end of the function
	} else {
		r.success = true
	}

	return r, nil
}

func (s *sandbox) Abort() error {
	s.aborted = true
	return nil
}
