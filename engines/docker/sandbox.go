package dockerengine

import (
	"context"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

type sandbox struct {
	engines.SandboxBase
	monitor         runtime.Monitor
	containerID     string
	resultSet       engines.ResultSet
	resultErr       error
	abortErr        error
	cancelLogs      context.CancelFunc
	containerExited chan struct{}
	tempStorage     runtime.TemporaryStorage
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
		containerID:     container.ID,
		tempStorage:     ts,
		containerExited: make(chan struct{}),
	}
	// HostConfig is ignored by the remote API and is only kept for
	// backward compatibility.
	_ = sb.e.client.StartContainer(container.ID, &docker.HostConfig{})
	go func() {
		_ = sb.e.client.Logs(docker.LogsOptions{
			Container:    container.ID,
			OutputStream: ioext.WriteNopCloser(sb.taskCtx.LogDrain()),
			ErrorStream:  ioext.WriteNopCloser(sb.taskCtx.LogDrain()),
			Follow:       true,
		})
		close(sbox.containerExited)
	}()

	return sbox, engines.ErrFeatureNotSupported
}

func (s *sandbox) WaitForResult() (engines.ResultSet, error) {
	<-s.containerExited
	// exitCode, err := s.e.client.WaitContainer(s.containerID)
	return nil, nil
}

func (s *sandbox) Kill() error {
	return nil
}

func (s *sandbox) Abort() error {
	return nil
}
