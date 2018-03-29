package dockerengine

import (
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

type configType struct {
	DockerSocket string `json:"dockerSocket"`
}

var configSchema = schematypes.Object{
	Properties: schematypes.Properties{
		"dockerSocket": schematypes.String{
			Title: "Docker Socket",
			Description: util.Markdown(`
				Path to the docker socket hosting the remote docker API.

				If not given the default value 'unix:///var/run/docker.sock' will be used.
			`),
		},
	},
}
