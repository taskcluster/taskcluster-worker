package nativeengine

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/native/system"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

type sandbox struct {
	engines.SandboxBase
	engine        *engine
	context       *runtime.TaskContext
	monitor       runtime.Monitor
	workingFolder runtime.TemporaryFolder
	user          *system.User
	process       *system.Process
	env           map[string]string
	resolve       atomics.Once // Guarding resultSet, resultErr and abortErr
	resultSet     *resultSet
	resultErr     error
	abortErr      error
	wg            atomics.WaitGroup
}

func newSandbox(b *sandboxBuilder) (s *sandbox, err error) {
	var user *system.User
	var workingFolder runtime.TemporaryFolder

	defer func() {
		if err != nil {
			if b.engine.config.CreateUser && user != nil {
				user.Remove()
			}

			if workingFolder != nil {
				_ = workingFolder.Remove()
			}
		}
	}()

	if b.engine.config.CreateUser {
		// Create temporary home folder for the task
		workingFolder, err = b.engine.environment.TemporaryStorage.NewFolder()
		if err != nil {
			err = fmt.Errorf("Failed to temporary folder, error: %s", err)
			b.monitor.Error(err)
			return
		}

		// Create temporary user account
		user, err = system.CreateUser(workingFolder.Path(), b.engine.groups)
		if err != nil {
			err = fmt.Errorf("Failed to create temporary system user, error: %s", err)
			return
		}
	} else {
		user, err = system.CurrentUser()
		if err != nil {
			return
		}
	}

	if b.payload.Context != "" {
		if err = fetchContext(b.payload.Context, user); err != nil {
			err = engines.NewMalformedPayloadError(
				fmt.Sprintf("Error downloading %s: %v", b.payload.Context, err),
			)
			b.context.LogError(err)
			return
		}
	}

	env := map[string]string{}
	for k, v := range b.env {
		env[k] = v
	}

	env["HOME"] = user.Home()
	env["USER"] = user.Name()
	env["LOGNAME"] = user.Name()

	// Start process
	debug("StartProcess: %v", b.payload.Command)
	process, err := system.StartProcess(system.ProcessOptions{
		Arguments:     b.payload.Command,
		Environment:   env,
		WorkingFolder: user.Home(),
		Owner:         user,
		Stdout:        ioext.WriteNopCloser(b.context.LogDrain()),
		// Stderr defaults to Stdout when not specified
	})
	if err != nil {
		// StartProcess provides human-readable error messages (see docs)
		// We'll convert it to a MalformedPayloadError
		err = engines.NewMalformedPayloadError(
			"Unable to start specified command: ", b.payload.Command, "error: ", err,
		)
		b.context.LogError(err)
		return
	}

	s = &sandbox{
		engine:        b.engine,
		context:       b.context,
		monitor:       b.monitor,
		workingFolder: workingFolder,
		user:          user,
		process:       process,
		env:           b.env,
	}

	go s.waitForTermination()

	return
}

func fetchContext(context string, user *system.User) error {
	// TODO: use future cache subsystem, when we have it
	// TODO: use the soon to be merged fetcher subsystem
	filename, err := util.Download(context, user.Home())
	if err != nil {
		return fmt.Errorf("Error downloading '%s': %v", context, err)
	}

	// TODO: verify if this will harm Windows
	// TODO: abstract this away in system package
	if err = os.Chmod(filename, 0700); err != nil {
		return fmt.Errorf("Error setting file '%s' permissions: %v", filename, err)
	}

	if err = system.ChangeOwner(filename, user); err != nil {
		return err
	}

	unpackedFile := ""
	switch filepath.Ext(filename) {
	case ".zip":
		err = runtime.Unzip(filename)
	case ".gz":
		unpackedFile, err = runtime.Gunzip(filename)
	}

	if err != nil {
		return fmt.Errorf("Error unpacking '%s': %v", context, err)
	}

	if filepath.Ext(unpackedFile) == ".tar" {
		err = runtime.Untar(unpackedFile)
		if err != nil {
			return fmt.Errorf("Error unpacking '%s': %v", context, err)
		}
	}

	return nil
}

func (s *sandbox) NewShell(command []string, tty bool) (engines.Shell, error) {
	// Increment shell counter, if draining we don't allow new shells
	if s.wg.Add(1) != nil {
		return nil, engines.ErrSandboxTerminated
	}

	debug("NewShell with: %v", command)
	shell, err := newShell(s, command, tty)
	if err != nil {
		debug("Failed to start shell, error: %s", err)
		s.wg.Done()
		return nil, engines.NewMalformedPayloadError(
			"Unable to spawn command: ", command, " error: ", err,
		)
	}

	// Wait for the shell to be done and decrement WaitGroup
	go func() {
		result, _ := shell.Wait()
		debug("Shell finished with: %v", result)
		s.wg.Done()
	}()

	return shell, nil
}

func (s *sandbox) waitForTermination() {
	// Wait for process to terminate
	success := s.process.Wait()
	debug("Process finished with: %v", success)

	// Wait for all shell to finish and prevent new shells from being created
	s.wg.WaitAndDrain()
	debug("All shells terminated")

	s.resolve.Do(func() {
		// Halt all other sub-processes
		if s.engine.config.CreateUser {
			system.KillByOwner(s.user)
		}

		// Create resultSet
		s.resultSet = &resultSet{
			engine:        s.engine,
			context:       s.context,
			monitor:       s.monitor,
			workingFolder: s.workingFolder,
			user:          s.user,
			success:       success,
		}
		s.abortErr = engines.ErrSandboxTerminated
	})
}

func (s *sandbox) WaitForResult() (engines.ResultSet, error) {
	// Wait for result and terminate
	s.resolve.Wait()
	return s.resultSet, s.resultErr
}

func (s *sandbox) Abort() error {
	s.resolve.Do(func() {
		debug("Aborting sandbox")

		// In case we didn't create a new user, killing
		// the children processes is the only safe way
		// to kill processes created by the task.
		system.KillProcessTree(s.process)

		if s.engine.config.CreateUser {
			// When we have a new user created, we can safely
			// kill any process owned by it.
			err := system.KillByOwner(s.user)
			if err != nil {
				s.monitor.Error("Failed to kill all processes by owner, error: ", err)
			}

			// Remove temporary user (this will panic if unsuccessful)
			s.user.Remove()
		}

		// Remove temporary home folder
		if s.workingFolder != nil {
			if err := s.workingFolder.Remove(); err != nil {
				s.monitor.Error("Failed to remove temporary home directory, error: ", err)
			}
		}

		// Set result
		s.resultErr = engines.ErrSandboxAborted
	})
	s.resolve.Wait()
	return s.abortErr
}
