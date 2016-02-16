package livelog

import "github.com/taskcluster/taskcluster-worker/runtime"

type (
	Config struct {
		LiveLogExe string `json:"liveLogExe"`
	}
)

func ConfigSchema() runtime.CompositeSchema {
	schema, err := runtime.NewCompositeSchema(
		"config",
		`
		{
		  "$schema": "http://json-schema.org/draft-04/schema#",
		  "additionalProperties": false,
		  "description": "LiveLog config",
		  "properties": {
		    "liveLogExe": {
		      "description": "The executable (.exe file) to run the livelog service",
		      "title": "LiveLogExecutable",
		      "type": "string"
		    }
		  },
		  "required": [
		    "liveLogExe"
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
