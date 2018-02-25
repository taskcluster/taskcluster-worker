package dockerengine

import (
	"context"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
	"time"
)

const dockerEngineKillTimeout = 5 * time.Second

type sandbox struct {
	engines.SandboxBase
	monitor     runtime.Monitor
	containerID string
	resultSet   engines.ResultSet
	resultErr   error
	abortErr    error
	cancelLogs  context.CancelFunc
	tempStorage runtime.TemporaryStorage
	resolve     atomics.Once
	client      *docker.Client
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
			"could not create container: %v", err)
	}
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
	}
	// HostConfig is ignored by the remote API and is only kept for
	// backward compatibility.
	_ = sbox.client.StartContainer(container.ID, &docker.HostConfig{})
	go func() {
		// logs is blocking, therefore started in a separate goroutine
		_ = sb.e.client.Logs(docker.LogsOptions{
			Container:    container.ID,
			OutputStream: ioext.WriteNopCloser(sb.taskCtx.LogDrain()),
			ErrorStream:  ioext.WriteNopCloser(sb.taskCtx.LogDrain()),
			Follow:       true,
		})
		sbox.resolve.Do(func() {
			exitCode, err := sbox.client.WaitContainer(sbox.containerID)
			success := exitCode == 0
			// Inspecting the container failed and the default return value
			// is 0, so assume that the task failed.
			if err != nil {
				sbox.monitor.ReportError(err)
				success = false
			}
			sbox.resultSet = newResultSet(success, sbox.containerID,
				sbox.client, sbox.tempStorage)
			sbox.abortErr = engines.ErrSandboxTerminated
		})
	}()

	return sbox, nil
}

func (s *sandbox) WaitForResult() (engines.ResultSet, error) {
	s.resolve.Wait()
	return s.resultSet, s.resultErr
}

// func (s *sandbox) waitForTermination() {
// 	// if killed or aborted containerExited will be closed by the goroutine
// 	// in start sandbox
// 	<-s.containerExited
// 	s.resolve.Do(func() {
// 		exitCode, err := s.client.WaitContainer(s.containerID)
// 		s.resultSet = newResultSet(exitCode == 0, s.containerID, s.client, s.tempStorage)
// 		if err != nil {
// 			s.resultSet = newResultSet(false, s.containerID, s.client, s.tempStorage)
// 		}
// 		s.abortErr = engines.ErrSandboxTerminated
// 	})
// }

func (s *sandbox) Kill() error {
	s.resolve.Do(func() {
		debug("Sandbox.Kill()")
		// maybe will use to timeout kill and send SIGKILL
		killCtx, cancelFunc := context.WithCancel(context.Background())

		go func() {
			_ = s.client.KillContainer(docker.KillContainerOptions{
				ID:      s.containerID,
				Signal:  docker.SIGTERM,
				Context: killCtx,
			})
		}()
		select {
		case <-killCtx.Done():
		case <-time.After(dockerEngineKillTimeout):
			cancelFunc()
			debug("container process is taking to long to shutdown. sending SIGKILL")
			_ = s.client.KillContainer(docker.KillContainerOptions{
				ID:     s.containerID,
				Signal: docker.SIGKILL,
			})
		}
		s.resultSet = newResultSet(false, s.containerID, s.client, s.tempStorage)
		s.abortErr = engines.ErrSandboxTerminated
	})
	s.resolve.Wait()
	return s.resultErr
}

func (s *sandbox) Abort() error {
	s.resolve.Do(func() {
		debug("Sandbox.Abort()")
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
		s.resultSet = newResultSet(false, s.containerID, nil, nil)
	})
	s.resolve.Wait()
	return s.abortErr
}
