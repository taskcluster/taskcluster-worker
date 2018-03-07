package dockerengine

import (
	"context"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
	"github.com/taskcluster/taskcluster-worker/runtime/caching"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

const dockerEngineKillTimeout = 5 * time.Second

type sandbox struct {
	engines.SandboxBase
	monitor     runtime.Monitor
	containerID string
	resultSet   engines.ResultSet
	resultErr   error
	abortErr    error
	tempStorage runtime.TemporaryFolder
	resolve     atomics.Once
	client      *docker.Client
	taskCtx     *runtime.TaskContext
	handle      *caching.Handle
}

func newSandbox(sb *sandboxBuilder) (*sandbox, error) {
	// create the container
	opts := docker.CreateContainerOptions{
		Config: sb.generateDockerConfig(),
	}

	container, err := sb.e.client.CreateContainer(opts)
	if err != nil {
		return nil, runtime.NewMalformedPayloadError(
			"could not create container: " + err.Error())
	}

	// create a temporary storage for use by resultSet
	ts, err := sb.e.Environment.TemporaryStorage.NewFolder()
	if err != nil {
		// unsure if this is the correct error type to return
		return nil, runtime.NewMalformedPayloadError(
			"could not create temporary storage")
	}
	sbox := &sandbox{
		containerID: container.ID,
		tempStorage: ts,
		client:      sb.e.client,
		taskCtx:     sb.taskCtx,
		handle:      sb.handle,
		monitor:     sb.monitor.WithPrefix("docker-sandbox").WithTag("taskID", sb.taskCtx.TaskID),
	}

	// attach to the container before starting so that we get all the logs
	_, err = sbox.client.AttachToContainerNonBlocking(docker.AttachToContainerOptions{
		Container:    container.ID,
		OutputStream: ioext.WriteNopCloser(sbox.taskCtx.LogDrain()),
		Logs:         true,
		Stdout:       true,
		Stderr:       true,
		Stream:       true,
	})

	// HostConfig is ignored by the remote API and is only kept for
	// backward compatibility.
	err = sbox.client.StartContainer(sbox.containerID, &docker.HostConfig{})
	if err != nil {
		return nil, runtime.ErrFatalInternalError
	}

	go sbox.wait()

	return sbox, nil
}

func (s *sandbox) WaitForResult() (engines.ResultSet, error) {
	s.resolve.Wait()
	return s.resultSet, s.resultErr
}

func (s *sandbox) wait() {
	exitCode, err := s.client.WaitContainer(s.containerID)
	s.resolve.Do(func() {
		s.resultSet = newResultSet(exitCode == 0, s.containerID, s.client,
			s.tempStorage, s.handle, s.monitor.WithPrefix("result-set"))
		if err != nil {
			s.resultSet = newResultSet(false, s.containerID, s.client, s.tempStorage,
				s.handle, s.monitor.WithPrefix("result-set"))
		}
		s.abortErr = engines.ErrSandboxTerminated
	})
}

func (s *sandbox) Kill() error {
	s.resolve.Do(func() {
		debug("Sandbox.Kill() for containerId: %s", s.containerID)
		s.resultErr = s.attemptGracefulTermination()

		// Create resultSet
		if s.resultErr == nil {
			s.resultSet = newResultSet(false, s.containerID, s.client,
				s.tempStorage, s.handle, s.monitor.WithPrefix("result-set"))
			s.abortErr = engines.ErrSandboxTerminated
		} else {
			s.dispose()
		}
	})
	s.resolve.Wait()
	return s.resultErr
}

func (s *sandbox) Abort() error {
	s.resolve.Do(func() {
		debug("Sandbox.Abort() for containerId: %s", s.containerID)
		s.attemptGracefulTermination()
		s.abortErr = s.dispose()
		s.resultErr = engines.ErrSandboxAborted
	})
	s.resolve.Wait()
	return s.abortErr
}

// attemptGracefulTermination will attempt a graceful termination of the
// container and ignore ContainerNotRunning errors.
func (s *sandbox) attemptGracefulTermination() error {
	hasErr := false

	// Send SIGTERM and give the container 30s to exit
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel() // always free the context
	err := s.client.KillContainer(docker.KillContainerOptions{
		ID:      s.containerID,
		Signal:  docker.SIGTERM,
		Context: ctx,
	})
	// Report errors if ContainerNotRunning or ctx was timed out
	if _, ok := err.(*docker.ContainerNotRunning); err != nil && !ok && ctx.Err() == nil {
		s.monitor.ReportError(err, "KillContainer with SIGTERM failed")
		// signal up the stack that something went wrong, this is not a successful kill
		hasErr = true
	}

	// Send SIGTERM and give docker 5 minutes to kill the container
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel() // always free the context
	err = s.client.KillContainer(docker.KillContainerOptions{
		ID:      s.containerID,
		Signal:  docker.SIGKILL,
		Context: ctx,
	})
	// Report errors other than ContainerNotRunning
	if _, ok := err.(*docker.ContainerNotRunning); err != nil && !ok {
		s.monitor.ReportError(err, "KillContainer with SIGTERM failed")
		hasErr = true
	}

	// Wait for container to exit
	_, err = s.client.WaitContainer(s.containerID)
	// Report errors other than ContainerNotRunning
	if _, ok := err.(*docker.ContainerNotRunning); err != nil && !ok {
		s.monitor.ReportError(err, "WaitContainer failed")
		hasErr = true
	}

	// If ErrNonFatalInternalError if there was an error of any kind, since all
	// errors here are not really fatal.
	if hasErr {
		return runtime.ErrNonFatalInternalError
	}
	return nil
}

// free all resources held by this sandbox
func (s *sandbox) dispose() error {
	hasErr := false

	// Remove the container
	err := s.client.RemoveContainer(docker.RemoveContainerOptions{
		ID:    s.containerID,
		Force: true,
	})
	if err != nil {
		s.monitor.ReportError(err, "failed to remove container in disposal of sandbox")
	}

	// Free image handle
	s.handle.Release()

	// Remove temporary storage
	if err = s.tempStorage.Remove(); err != nil {
		s.monitor.ReportError(err, "failed to remove temporary storage in disposal of sandbox")
	}

	// If ErrNonFatalInternalError if there was an error of any kind
	if hasErr {
		return runtime.ErrNonFatalInternalError
	}
	return nil
}
