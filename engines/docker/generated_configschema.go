package docker

import "github.com/taskcluster/taskcluster-worker/runtime"

type (
	Config struct {
		RootVolume string `json:"rootVolume"`
	}
)

func ConfigSchema() runtime.CompositeSchema {
	schema, err := runtime.NewCompositeSchema(
		"config",
		`
		{
		  "$schema": "http://json-schema.org/draft-04/schema#",
		  "additionalProperties": false,
		  "description": "Config applicable to docker engine",
		  "properties": {
		    "rootVolume": {
		      "description": "Root Volume blah blah",
		      "title": "Root Volume",
		      "type": "string"
		    }
		  },
		  "required": [
		    "rootVolume"
		  ],
		  "title": "Config",
		  "type": "object"
		}
		`,
		true,
		func() interface{} { return &Config{} },
	)
	if err != nil {
		panic(err)
	}
	return schema
}
