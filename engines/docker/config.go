// +build linux

package dockerengine

import (
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

type configType struct {
	DockerSocket string `json:"dockerSocket"`
	Privileged   string `json:"privileged"`
}

const (
	privilegedAlways = "always"
	privilegedAllow  = "allow"
	privilegedNever  = "never"
)

var configSchema = schematypes.Object{
	Properties: schematypes.Properties{
		"dockerSocket": schematypes.String{
			Title: "Docker Socket",
			Description: util.Markdown(`
				Path to the docker socket hosting the remote docker API.

				If not given the default value 'unix:///var/run/docker.sock' will be used.
			`),
		},
		"privileged": schematypes.StringEnum{
			Title: "Privileged Mode",
			Description: util.Markdown(`
				Allow task containers to run in privileged mode.

				This option can take one of the 3 values:
				 * 'always', run the task container in privileged mode regardless of
				   what scopes the task in question has.
				 * 'allow', the task container to run in privileged mode, if
				   'task.payload.privileged == true', and the task has the scope
					 'worker:privileged:<provisionerId>/<workerType>'.
				 * 'never', run task containers in privileged mode.

				If in doubt use 'never' and enable privileged mode when needed.
			`),
			Options: []string{
				privilegedAlways,
				privilegedAllow,
				privilegedNever,
			},
		},
	},
	Required: []string{
		"privileged",
	},
}
