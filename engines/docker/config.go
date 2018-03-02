package dockerengine

import (
	schematypes "github.com/taskcluster/go-schematypes"
)

type configType struct {
	DockerSocket   string `json:"dockerSocket"`
	MaxConcurrency int    `json:"maxConcurrency"`
}

var configSchema = schematypes.Object{
	Properties: schematypes.Properties{
		"dockerSocket": schematypes.String{
			Title: "Docker Endpoint",
			Description: "dockerEndpoint is the endpoint to use for communicating\n" +
				"with the Docker daemon.",
		},
		"maxConcurrency": schematypes.Integer{
			Title: "Max Concurrency",
			Description: "maxConcurrency defines the maximum number of tasks \n" +
				"that may run concurrently on the worker.",
			Minimum: 1,
		},
	},
}
