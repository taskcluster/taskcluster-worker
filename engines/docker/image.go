package dockerengine

import (
	docker "github.com/fsouza/go-dockerclient"
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime/fetcher"
)

var imageSchema = schematypes.OneOf{
	fetcher.Artifact.Schema(),
	schematypes.String{
		Title:       "Docker image",
		Description: "The docker image to pull",
	},
}

func pullImage(client *dockerClient, imagePayload interface{}) (*docker.Image, error) {
	switch i := imagePayload.(type) {
	case string:
		return client.PullImageFromRepository(i)
	default:
		return client.PullImageFromArtifact(imagePayload)
	}
}
