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
	tempStorage runtime.TemporaryStorage
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

	debug("creating container for task: %s", sb.taskCtx.TaskID)

	container, err := sb.e.client.CreateContainer(opts)
	if err != nil {
		return nil, runtime.NewMalformedPayloadError(
			"could not create container: " + err.Error())
	}
	debug("created container")

	// create a temporary storage for use by resultSet
	debug("creating temporary storage")
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
	debug("attached to container (non blocking)")

	// HostConfig is ignored by the remote API and is only kept for
	// backward compatibility.
	err = sbox.client.StartContainer(sbox.containerID, &docker.HostConfig{})
	if err != nil {
		return nil, runtime.ErrFatalInternalError
	}
	debug("started container")

	go sbox.wait()

	return sbox, nil
}

func (s *sandbox) WaitForResult() (engines.ResultSet, error) {
	s.resolve.Wait()
	debug("result generated")
	return s.resultSet, s.resultErr
}

func (s *sandbox) wait() {
	exitCode, err := s.client.WaitContainer(s.containerID)
	s.resolve.Do(func() {
		s.resultSet = newResultSet(exitCode == 0, s.containerID, s.client, s.tempStorage, s.handle)
		if err != nil {
			s.resultSet = newResultSet(false, s.containerID, s.client, s.tempStorage, s.handle)
		}
		s.abortErr = engines.ErrSandboxTerminated
	})
}

func (s *sandbox) Kill() error {
	s.resolve.Do(func() {
		debug("Sandbox.Kill()")
		// maybe will use to timeout kill and send SIGKILL
		killCtx, cancelFunc := context.WithCancel(context.Background())
		kchan := make(chan struct{}, 1)

		go func() {
			_ = s.client.KillContainer(docker.KillContainerOptions{
				ID:      s.containerID,
				Signal:  docker.SIGTERM,
				Context: killCtx,
			})
			select {
			case <-kchan:
			default:
				close(kchan)
			}
		}()
		select {
		case <-kchan:
		case <-time.After(dockerEngineKillTimeout):
			cancelFunc()
			debug("container process is taking to long to shutdown. sending SIGKILL")
			_ = s.client.KillContainer(docker.KillContainerOptions{
				ID:     s.containerID,
				Signal: docker.SIGKILL,
			})
		}
		s.resultSet = newResultSet(false, s.containerID, s.client, s.tempStorage, s.handle)
		s.abortErr = engines.ErrSandboxTerminated
		debug("killed container")
	})
	s.resolve.Wait()
	return s.resultErr
}

func (s *sandbox) Abort() error {
	s.resolve.Do(func() {
		debug("Sandbox.Abort()")
		// debug("Sandbox.Abort()")
		killOpts := docker.KillContainerOptions{
			ID:     s.containerID,
			Signal: docker.SIGKILL,
		}
		err := s.client.KillContainer(killOpts)
		if err != nil {
			s.monitor.ReportError(err)
		}
		_ = s.tempStorage.(runtime.TemporaryFolder).Remove()
		s.abortErr = engines.ErrSandboxAborted
		s.resultSet = newResultSet(false, s.containerID, nil, nil, s.handle)
	})
	s.resolve.Wait()
	return s.abortErr
}
