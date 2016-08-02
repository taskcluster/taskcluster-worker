// +build darwin

package osxnative

import (
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
)

type resultset struct {
	engines.ResultSetBase
	taskUser user
	context  *runtime.TaskContext
	success  bool
	engine   *engine
}

var pathMatcher *regexp.Regexp

// Check if the path is valid. The only valid paths are those
// inside user's home directory and in /tmp/
func (r resultset) validPath(home string, pathName string) bool {
	pathMatcher = regexp.MustCompile("^(" + home + "|/tmp)(/.*)?$")

	if !filepath.IsAbs(pathName) {
		pathName = filepath.Join(home, pathName)
	}

	absPath, err := filepath.Abs(pathName)

	if err != nil {
		r.context.LogError(pathName, err)
		return false
	}

	absPath = filepath.Clean(absPath)

	return pathMatcher.MatchString(absPath)
}

func (r resultset) Success() bool {
	return r.success
}

func (r resultset) ExtractFile(path string) (ioext.ReadSeekCloser, error) {
	cwd := getWorkingDir(r.taskUser, r.context)
	if !r.validPath(cwd, path) {
		return nil, engines.NewMalformedPayloadError(path + " is invalid")
	}

	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}

	file, err := os.Open(path)

	if err != nil {
		r.context.LogError(err)
		return nil, engines.ErrResourceNotFound
	}

	return file, nil
}

func (r resultset) ExtractFolder(path string, handler engines.FileHandler) error {
	cwd := getWorkingDir(r.taskUser, r.context)
	if !r.validPath(cwd, path) {
		return engines.NewMalformedPayloadError(path + " is invalid")
	}

	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}

	return filepath.Walk(path, func(p string, info os.FileInfo, e error) error {
		if e != nil {
			r.context.LogError(e)
			return engines.ErrResourceNotFound
		}

		if !info.IsDir() {
			file, err := os.Open(p)
			if err != nil {
				r.context.LogError(err)
				return engines.ErrResourceNotFound
			}

			err = handler(p, file)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (r resultset) Dispose() error {
	err := r.taskUser.delete()
	if err != nil {
		r.engine.log.WithField("user", r.taskUser.name).WithError(err).Error("Error removing user")
		exitError, ok := err.(*exec.ExitError)
		if ok {
			r.engine.log.Error(string(exitError.Stderr))
		}

		return engines.ErrNonFatalInternalError
	}

	return nil
}
