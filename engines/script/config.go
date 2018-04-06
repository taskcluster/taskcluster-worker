package scriptengine

import (
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

type configType struct {
	Command []string `json:"command"`
	Schema  struct {
		Type       string                 `json:"type"`
		Properties map[string]interface{} `json:"properties"`
		Required   []string               `json:"required"`
	} `json:"schema"`
}

var configSchema = schematypes.Object{
	Title:       "Script Engine Configuration",
	Description: `Configuration properties for the 'scriptengine'.`,
	Properties: schematypes.Properties{
		"command": schematypes.Array{
			Title: "Command to Execute",
			Description: util.Markdown(`
				Script and arguments to execute. This script will be fed
				a JSON string that matches the schema configured over 'stdin'.

				Output from the script over 'stdout' will be uploaded as task log.
				Output from the script over 'stderr' will be prefixed "[worker:error]"
				and merged with task log.

				The script will be executed with a temporary folder as
				_working directory_, this folder can be used for temporary storage and
				will be cleared between tasks.

				Files and folder stored in './artifacts/' relative to the
				_working directory_ will be uploaded as artifacts from
				the script. Hence, to make a public tar-ball artifact you create
				'./artifact/public/my-build.tar.gz' which will be uploaded as an
				artifact named 'public/my-build.tar.gz'.

				Exit codes from the script will be intepreted as follows:
				 * '0', task was executed successfully,
				 * '1', task was executed but failed,
				 * '2', task payload was not permitted, errors should be printed to stderr,
				 * '3', script had a non-fatal error, task is resolved exception
				 * '4', script had a fatal error, task is resolved exception, and
				   the worker crashes.
			`),
			Items: schematypes.String{},
		},
		"schema": schematypes.Object{
			Title: "Payload Schema",
			Description: util.Markdown(`
				JSON schema for 'task.payload'. A JSON string matching this
				schema will be piped to the script command over stdin.
			`),
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
	},
	Required: []string{
		"command",
		"schema",
	},
}
