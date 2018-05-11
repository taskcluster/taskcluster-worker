// +build docker

package dockerengine

import (
	"testing"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/require"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

const dockerSocket = "unix:///var/run/docker.sock"

func newDockerClient(t *testing.T) *dockerClient {
	c, err := docker.NewClient(dockerSocket)
	require.NoError(t, err)

	return &dockerClient{
		Client: c,
	}
}

func testDownloadByTag(t *testing.T, imageName string) {
	taskContext, _, err := runtime.NewTaskContext("/tmp/log.txt", runtime.TaskInfo{
		TaskID: "mytaskid",
	})
	require.NoError(t, err)

	client := newDockerClient(t)

	image, err := client.PullImageFromRepository(taskContext, imageName)
	require.NoError(t, err)

	defer client.RemoveImageExtended(image.ID, docker.RemoveImageOptions{
		Force:   true,
		Context: taskContext,
	})
}

func TestDownloadByRepoTagName(t *testing.T) {
	testDownloadByTag(t, "alpine:3.6")
}

func TestDownloadByHash(t *testing.T) {
	testDownloadByTag(t, "taskcluster/worker-ci@sha256:125150396fbac3b0e8caf5880ebf10fc952bc2357d1509e9a6bee6872838fa5e")
}

func TestDownloadByArtifact(t *testing.T) {
	taskContext, _, err := runtime.NewTaskContext("/tmp/log.txt", runtime.TaskInfo{
		TaskID: "mytaskid",
	})
	require.NoError(t, err)

	client := newDockerClient(t)

	image, err := client.PullImageFromArtifact(taskContext, map[string]interface{}{
		"taskId":   "Q4I6ROTUS-OvNrDDMaq_vw",
		"runId":    0,
		"artifact": "public/image.tar.zst",
	})
	require.NoError(t, err)

	require.NoError(t, client.RemoveImageExtended(image.ID, docker.RemoveImageOptions{
		Force:   true,
		Context: taskContext,
	}))
}
