package scriptengine

import schematypes "github.com/taskcluster/go-schematypes"

type configType struct {
	Script     []string `json:"script"`
	Expiration int      `json:"expiration"`
	Schema     struct {
		Type       string                 `json:"type"`
		Properties map[string]interface{} `json:"properties"`
		Required   []string               `json:"required"`
	} `json:"schema"`
}

var configSchema = schematypes.Object{
	MetaData: schematypes.MetaData{
		Title:       "Script Engine Configuration",
		Description: `Configuration properties for the 'scriptengine'.`,
	},
	Properties: schematypes.Properties{
		"script": schematypes.Array{
			MetaData: schematypes.MetaData{
				Title: "Script to Execute",
				Description: `Script and arguments to execute. This script will be fed
        a JSON string that matches the schema configured over stdin.`,
			},
			Items: schematypes.String{},
		},
		"schema": schematypes.Object{
			MetaData: schematypes.MetaData{
				Title: "Payload Schema",
				Description: `JSON schema for 'task.payload'. A JSON string matching
        this schema will be piped to the script command over stdin.`,
			},
			Properties: schematypes.Properties{
				"type":       schematypes.StringEnum{Options: []string{"object"}},
				"properties": schematypes.Object{AdditionalProperties: true},
				"required":   schematypes.Array{Items: schematypes.String{}},
			},
			Required: []string{
				"type",
				"properties",
				"required",
			},
		},
		"expiration": schematypes.Integer{
			MetaData: schematypes.MetaData{
				Title:       "Artifact Expiration",
				Description: "Number of days before artifact expiration.",
			},
			Minimum: 1,
			Maximum: 365,
		},
	},
	Required: []string{
		"script",
		"schema",
		"expiration",
	},
}
