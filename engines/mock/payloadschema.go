package mockengine

import schematypes "github.com/taskcluster/go-schematypes"

type payloadType struct {
	Delay    int    `json:"delay"`
	Function string `json:"function"`
	Argument string `json:"argument"`
}

var payloadSchema = schematypes.Object{
	Properties: schematypes.Properties{
		"delay": schematypes.Integer{
			MetaData: schematypes.MetaData{
				Title: "Execution Time",
				Description: `MockEngine is used to mock a real engine, towards that
				end you can specify how long time the MockEngine should sleep before
				reporting the execution as done.`,
			},
			Minimum: 0,
			Maximum: 5 * 60 * 1000,
		},
		"function": schematypes.StringEnum{
			MetaData: schematypes.MetaData{
				Title:       "Function to Execute",
				Description: "MockEngine supports running one of these pre-defined functions.",
			},
			Options: []string{
				"true",
				"false",
				"set-volume",
				"get-volume",
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
			MetaData: schematypes.MetaData{
				Title: "Argument to be given to function",
				Description: `This argument will be passed to function, notice that
				not all functions take an argument and may just choose to ignore it.`,
			},
			MaximumLength: 255,
		},
	},
	Required: []string{
		"delay",
		"function",
		"argument",
	},
}
