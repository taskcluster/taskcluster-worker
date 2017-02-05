package nativeengine

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/native/system"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

type resultSet struct {
	engines.ResultSetBase
	engine     *engine
	context    *runtime.TaskContext
	log        *logrus.Entry
	homeFolder runtime.TemporaryFolder
	user       *system.User
	success    bool
}

func (r *resultSet) Success() bool {
	return r.success
}

func (r *resultSet) ExtractFile(path string) (ioext.ReadSeekCloser, error) {
	// Evaluate symlinks
	p, err := filepath.EvalSymlinks(filepath.Join(r.homeFolder.Path(), path))
	if err != nil {
		if _, ok := err.(*os.PathError); ok {
			return nil, engines.ErrResourceNotFound
		}
		return nil, engines.NewMalformedPayloadError(
			"Unable to evaluate path: ", path,
		)
	}

	// Cleanup the path
	p = filepath.Clean(p)

	prefix, err := filepath.EvalSymlinks(r.homeFolder.Path() + string(filepath.Separator))
	if err != nil {
		panic(err)
	}

	// Check that p is inside homeFolder
	if !strings.HasPrefix(p, prefix) {
		return nil, engines.ErrResourceNotFound
	}

	// Stat the file to make sure it's a file
	info, err := os.Lstat(p)
	if err != nil {
		return nil, engines.ErrResourceNotFound
	}
	// Don't allow anything that isn't a plain file
	if !ioext.IsPlainFileInfo(info) {
		return nil, engines.ErrResourceNotFound
	}

	// Open file
	f, err := os.Open(p)
	if err != nil {
		return nil, engines.ErrResourceNotFound
	}

	return f, nil
}

func (r *resultSet) ExtractFolder(path string, handler engines.FileHandler) error {
	// Evaluate symlinks
	p, err := filepath.EvalSymlinks(filepath.Join(r.homeFolder.Path(), path))
	if err != nil {
		if _, ok := err.(*os.PathError); ok {
			return engines.ErrResourceNotFound
		}
		return engines.NewMalformedPayloadError(
			"Unable to evaluate path: ", path,
		)
	}

	// Cleanup the path
	p = filepath.Clean(p)

	prefix, err := filepath.EvalSymlinks(r.homeFolder.Path() + string(filepath.Separator))
	if err != nil {
		panic(err)
	}

	// Check that p is inside homeFolder
	if !strings.HasPrefix(p, prefix) {
		return engines.ErrResourceNotFound
	}

	first := true
	return filepath.Walk(p, func(abspath string, info os.FileInfo, err error) error {
		// If there is a path error, on the first call then the folder is missing
		if _, ok := err.(*os.PathError); ok && first {
			return engines.ErrResourceNotFound
		}
		first = false

		// Ignore folder we can't walk (probably a permission issues)
		if err != nil {
			return nil
		}

		// Skip anything that isn't a plain file
		if !ioext.IsPlainFileInfo(info) {
			return nil
		}

		// If we can't construct relative file path this internal error, we'll skip
		relpath, err := filepath.Rel(p, abspath)
		if err != nil {
			// TODO: Send error to sentry
			r.log.Errorf(
				"ExtractFolder from %s, filepath.Rel('%s', '%s') returns error: %s",
				path, p, abspath, err,
			)
			return nil
		}

		f, err := os.Open(abspath)
		if err != nil {
			// file must have been deleted as we tried to open it
			// that makes no sense, but who knows...
			return nil
		}

		// If handler returns an error we return ErrHandlerInterrupt
		if handler(filepath.ToSlash(relpath), f) != nil {
			return engines.ErrHandlerInterrupt
		}
		return nil
	})
}

func (r *resultSet) Dispose() error {
	// if we didn't create a user for this sandbox, we shouldn't destroy it either.
	// TODO: why is this cleanup here, and not in Sandbox?
	if !r.engine.config.CreateUser {
		return nil
	}

	// Halt all other sub-processes
	err := system.KillByOwner(r.user)
	if err != nil {
		r.log.Error("Failed to kill all processes by owner, error: ", err)
	}

	// Remove temporary user (this will panic if unsuccessful)
	r.user.Remove()

	// Remove temporary home folder
	if rerr := r.homeFolder.Remove(); rerr != nil {
		r.log.Error("Failed to remove temporary home directory, error: ", rerr)
		err = rerr
	}

	return err
}
