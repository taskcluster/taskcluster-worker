package nativeengine

import schematypes "github.com/taskcluster/go-schematypes"

type payload struct {
	Command []string `json:"command"`
}

var payloadSchema = schematypes.Object{
	Properties: schematypes.Properties{
		"command": schematypes.Array{
			MetaData: schematypes.MetaData{
				Title:       "Command",
				Description: "Command to execute",
			},
			Items: schematypes.String{},
		},
	},
	Required: []string{"command"},
}
