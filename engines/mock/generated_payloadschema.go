package mockengine

import "github.com/taskcluster/taskcluster-worker/runtime"

type (
	Payload struct {
		Argument string `json:"argument"`

		Delay int `json:"delay"`

		Function string `json:"function"`
	}
)

func PayloadSchema() runtime.CompositeSchema {
	schema, err := runtime.NewCompositeSchema(
		"start",
		`
		{
		  "$schema": "http://json-schema.org/draft-04/schema#",
		  "additionalProperties": false,
		  "properties": {
		    "argument": {
		      "type": "string"
		    },
		    "delay": {
		      "type": "integer"
		    },
		    "function": {
		      "enum": [
		        "true",
		        "false",
		        "set-volume",
		        "get-volume",
		        "ping-proxy",
		        "write-log",
		        "write-error-log"
		      ],
		      "type": "string"
		    }
		  },
		  "required": [
		    "delay",
		    "function",
		    "argument"
		  ],
		  "title": "Payload",
		  "type": "object"
		}
		`,
		true,
		func() interface{} {
			return &Payload{}
		},
	)
	if err != nil {
		panic(err)
	}
	return schema
}
