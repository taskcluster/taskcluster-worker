package osxnative

import (
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type resultset struct {
	engines.ResultSetBase
	context *runtime.TaskContext
	success bool
}

var pathMatcher *regexp.Regexp

func init() {
	home := os.Getenv("HOME")
	if strings.HasPrefix(home, "/") {
		// remove trailing "/"
		home = home[:len(home)]
	}

	pathMatcher = regexp.MustCompile("^(" + home + "|/tmp)(/.*)?$")
}

func newResultSet(context *runtime.TaskContext, success bool) resultset {
	return resultset{
		ResultSetBase: engines.ResultSetBase{},
		context:       context,
		success:       success,
	}
}

// Check if the path is valid. The only valid paths are those
// inside user's home directory and in /tmp/
func (r resultset) validPath(pathName string) bool {
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
	if !r.validPath(path) {
		return nil, engines.NewMalformedPayloadError(path + " is invalid")
	}

	file, err := os.Open(path)

	if err != nil {
		r.context.LogError(err)
		return nil, engines.ErrResourceNotFound
	}

	return file, nil
}

func (r resultset) ExtractFolder(path string, handler engines.FileHandler) error {
	if !r.validPath(path) {
		return engines.NewMalformedPayloadError(path + " is invalid")
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
