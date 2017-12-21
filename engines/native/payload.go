package nativeengine

import (
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

type payload struct {
	Command []string `json:"command"`
	Context string   `json:"context"`
}

var payloadSchema = schematypes.Object{
	Properties: schematypes.Properties{
		"command": schematypes.Array{
			Title:       "Command",
			Description: "Command to execute",
			Items:       schematypes.String{},
		},
		"context": schematypes.URI{
			Title: "Task Context",
			Description: util.Markdown(`
				Optional URL for a gzipped tar-ball to downloaded
				and extracted in the 'HOME' directory for running the command.
			`),
		},
	},
	Required: []string{"command"},
}
