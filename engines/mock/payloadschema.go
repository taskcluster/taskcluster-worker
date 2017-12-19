package mockengine

import (
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

type payloadType struct {
	Delay    int    `json:"delay"`
	Function string `json:"function"`
	Argument string `json:"argument"`
}

var payloadSchema = schematypes.Object{
	Properties: schematypes.Properties{
		"delay": schematypes.Integer{
			Title: "Execution Time",
			Description: util.Markdown(`
				MockEngine is used to mock a real engine, towards that end you
				can specify how long time the MockEngine should sleep before
				reporting the execution as done.
			`),
			Minimum: 0,
			Maximum: 5 * 60 * 1000,
		},
		"function": schematypes.StringEnum{
			Title:       "Function to Execute",
			Description: "MockEngine supports running one of these pre-defined functions.",
			Options: []string{
				"true",
				"false",
				"write-volume",
				"read-volume",
				"get-url",
				"ping-proxy",
				"write-log",
				"write-error-log",
				"write-log-sleep",
				"write-files",
				"print-env-var",
				"malformed-payload-initial",
				"malformed-payload-after-start",
				"fatal-internal-error",
				"nonfatal-internal-error",
				"stopNow-sleep",
			},
		},
		"argument": schematypes.String{
			Title: "Argument to be given to function",
			Description: util.Markdown(`
				This argument will be passed to function, notice that not all
				functions take an argument and may just choose to ignore it.
			`),
			MaximumLength: 255,
		},
	},
	Required: []string{
		"delay",
		"function",
		"argument",
	},
}
