package scriptengine

import (
	"mime"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
)

const artifactFolder = "artifacts"

type sandbox struct {
	engines.SandboxBase
	context     *runtime.TaskContext
	engine      *engine
	cmd         *exec.Cmd
	folder      runtime.TemporaryFolder
	resolve     atomics.Once
	resultSet   engines.ResultSet
	resultError error
	resultAbort error
	monitor     runtime.Monitor
	aborted     atomics.Bool
	done        chan struct{}
}

func (s *sandbox) run() {
	err := s.cmd.Wait()
	success := err == nil

	// if error wasn't because script exited non-zero, then we have a problem
	if _, ok := err.(*exec.ExitError); err != nil && !ok {
		s.monitor.Error("Script execution failed, error: ", err)
	}

	// Upload artifacts if not aborted
	if !s.aborted.Get() {
		err2 := s.uploadArtifacts()
		if err2 != nil {
			success = false
			s.context.LogError("Failed to upload artifacts")
			s.monitor.Warn("Failed to upload artifacts, error: ", err2)
		}
	} else {
		success = false
	}
	close(s.done)

	s.resolve.Do(func() {
		// Remove folder, log error
		err := s.folder.Remove()
		if err != nil {
			s.monitor.Errorf("Failed to remove temporary folder, error: %s", err)
		}

		s.resultSet = &resultSet{success: success}
		s.resultAbort = engines.ErrSandboxTerminated
	})
}

func (s *sandbox) WaitForResult() (engines.ResultSet, error) {
	s.resolve.Wait()
	return s.resultSet, s.resultError
}

func (s *sandbox) Abort() error {
	s.resolve.Do(func() {
		// Abort artifact upload
		s.aborted.Set(true)

		// Discard error from Kill() as we're racing with termination
		_ = s.cmd.Process.Kill()

		// Wait for artifact upload to be aborted
		<-s.done

		// Remove folder, log error
		err := s.folder.Remove()
		if err != nil {
			s.monitor.Errorf("Failed to remove temporary folder, error: %s", err)
		}
		s.resultError = engines.ErrSandboxAborted
	})
	s.resolve.Wait()
	return s.resultAbort
}

func (s *sandbox) uploadArtifacts() error {
	folder := filepath.Join(s.folder.Path(), artifactFolder)
	return filepath.Walk(folder, func(p string, info os.FileInfo, err error) error {
		// Abort if there is an error
		if err != nil {
			return err
		}

		// Skip folders
		if info.IsDir() {
			return nil
		}

		// Guess mimetype
		mimeType := mime.TypeByExtension(filepath.Ext(p))
		if mimeType == "" {
			// application/octet-stream is the mime type for "unknown"
			mimeType = "application/octet-stream"
		}

		// Open file
		f, err := os.Open(p)
		if err != nil {
			return err
		}

		// Find filename
		name, _ := filepath.Rel(folder, p)

		// Ensure expiration is no later than task.expires
		expires := time.Now().Add(time.Duration(s.engine.config.Expiration) * 24 * time.Hour)
		if time.Time(s.context.Expires).Before(expires) {
			expires = time.Time(s.context.Expires)
		}

		// Upload artifact
		err = s.context.UploadS3Artifact(runtime.S3Artifact{
			Name:     filepath.ToSlash(name),
			Mimetype: mimeType,
			Expires:  expires,
			Stream:   f,
		})

		// Ensure that we close the file
		cerr := f.Close()

		// Return first error, if any
		if err != nil {
			err = cerr
		}
		return err
	})
}
