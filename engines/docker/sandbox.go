package dockerengine

import (
	"context"
	"fmt"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/pkg/errors"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/docker/network"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
	"github.com/taskcluster/taskcluster-worker/runtime/caching"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

const dockerEngineKillTimeout = 5 * time.Second

type sandbox struct {
	engines.SandboxBase
	monitor       runtime.Monitor
	containerID   string
	resultSet     engines.ResultSet
	resultErr     error
	abortErr      error
	storage       runtime.TemporaryFolder
	resolve       atomics.Once
	docker        *docker.Client
	taskCtx       *runtime.TaskContext
	imageHandle   *caching.Handle
	networkHandle *network.Handle
}

func newSandbox(sb *sandboxBuilder) (*sandbox, error) {
	monitor := sb.monitor.WithTag("struct", "sandbox")

	// Get an isolated network, forwarding requests to gateway to proxyMux
	networkHandle, err := sb.e.networks.GetNetwork(&proxyMux{
		Proxies:     sb.proxies,
		TaskContext: sb.taskCtx,
	})
	if err != nil {
		// Any error here is a fatal error
		return nil, errors.Wrap(err, "docker.CreateNetwork failed")
	}

	// Create the container
	container, err := sb.e.docker.CreateContainer(docker.CreateContainerOptions{
		Config: &docker.Config{
			Cmd:          sb.payload.Command,
			Image:        buildImageName(sb.image.Repository, sb.image.Tag),
			Env:          *sb.env,
			AttachStdout: true,
			AttachStderr: true,
			Labels: map[string]string{
				"taskId": sb.taskCtx.TaskID,
			},
		},
		HostConfig: &docker.HostConfig{
			Privileged: sb.payload.Privileged,
			// gateway IP is also the host machine that we're listening for requests
			// to the proxies added to proxyMux above..
			ExtraHosts: []string{fmt.Sprintf("taskcluster:%s", networkHandle.Gateway())},
		},
		NetworkingConfig: &docker.NetworkingConfig{
			EndpointsConfig: map[string]*docker.EndpointConfig{
				networkHandle.NetworkID(): {},
			},
		},
	})
	if err != nil {
		return nil, runtime.NewMalformedPayloadError(
			"could not create container: " + err.Error())
	}

	// create a temporary storage for use by resultSet
	storage, err := sb.e.Environment.TemporaryStorage.NewFolder()
	if err != nil {
		monitor.ReportError(err, "failed to create temporary folder")
		return nil, runtime.ErrFatalInternalError
	}
	s := &sandbox{
		containerID:   container.ID,
		storage:       storage,
		docker:        sb.e.docker,
		taskCtx:       sb.taskCtx,
		imageHandle:   sb.imageHandle,
		networkHandle: networkHandle,
		monitor: monitor.WithTags(map[string]string{
			"containerId": container.ID,
			"networkId":   networkHandle.NetworkID(),
		}),
	}

	// attach to the container before starting so that we get all the logs
	_, err = s.docker.AttachToContainerNonBlocking(docker.AttachToContainerOptions{
		Container:    container.ID,
		OutputStream: ioext.WriteNopCloser(s.taskCtx.LogDrain()), // TODO: wait for close() before resolving task in s.wait()
		Logs:         true,
		Stdout:       true,
		Stderr:       true,
		Stream:       true,
	})
	if err != nil {
		return nil, errors.Wrap(err, "docker.AttachToContainerNonBlocking() failed")
	}

	// HostConfig is ignored by the remote API and is only kept for
	// backward compatibility.
	err = s.docker.StartContainer(s.containerID, &docker.HostConfig{})
	if err != nil {
		return nil, errors.Wrap(err, "docker.StartContainer failed")
	}

	go s.wait()

	return s, nil
}

func (s *sandbox) WaitForResult() (engines.ResultSet, error) {
	s.resolve.Wait()
	return s.resultSet, s.resultErr
}

func (s *sandbox) wait() {
	exitCode, err := s.docker.WaitContainer(s.containerID)
	s.resolve.Do(func() {
		if err != nil {
			incidentID := s.monitor.ReportError(err, "docker.WaitContainer failed")
			s.taskCtx.LogError("internal error waiting for container, incidentId:", incidentID)
			s.resultErr = runtime.ErrNonFatalInternalError
			s.abortErr = engines.ErrSandboxTerminated
			return
		}
		s.resultSet = &resultSet{
			success:       exitCode == 0,
			containerID:   s.containerID,
			docker:        s.docker,
			monitor:       s.monitor.WithTag("struct", "resultSet"),
			storage:       s.storage,
			context:       s.taskCtx,
			imageHandle:   s.imageHandle,
			networkHandle: s.networkHandle,
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
			s.resultSet = &resultSet{
				success:       false,
				containerID:   s.containerID,
				docker:        s.docker,
				monitor:       s.monitor.WithTag("struct", "resultSet"),
				storage:       s.storage,
				context:       s.taskCtx,
				imageHandle:   s.imageHandle,
				networkHandle: s.networkHandle,
			}
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
	err := s.docker.KillContainer(docker.KillContainerOptions{
		ID:      s.containerID,
		Signal:  docker.SIGTERM,
		Context: ctx,
	})
	// Report error if not ContainerNotRunning and ctx was not timed out
	if _, ok := err.(*docker.ContainerNotRunning); err != nil && !ok && ctx.Err() == nil {
		s.monitor.ReportError(err, "KillContainer with SIGTERM failed")
		// signal up the stack that something went wrong, this is not a successful kill
		hasErr = true
	}

	// Send SIGTERM and give docker 5 minutes to kill the container
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel() // always free the context
	err = s.docker.KillContainer(docker.KillContainerOptions{
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
	_, err = s.docker.WaitContainer(s.containerID)
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
	err := s.docker.RemoveContainer(docker.RemoveContainerOptions{
		ID:            s.containerID,
		Force:         true, // Kill anything still running in the container
		RemoveVolumes: true, // Remove any volumes automatically created with the container (VOLUME in docker image)
	})
	if err != nil {
		s.monitor.ReportError(err, "failed to remove container in disposal of sandbox")
		hasErr = true
	}

	// Free image handle
	s.imageHandle.Release()

	// Remove temporary storage
	if err = s.storage.Remove(); err != nil {
		s.monitor.ReportError(err, "failed to remove temporary storage in disposal of sandbox")
		hasErr = true
	}

	// Release the network
	s.networkHandle.Release()

	// If ErrNonFatalInternalError if there was an error of any kind
	if hasErr {
		return runtime.ErrNonFatalInternalError
	}
	return nil
}
