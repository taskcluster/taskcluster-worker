package qemuguesttools

import (
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

type config struct {
	Entrypoint []string          `json:"entrypoint,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
	Shell      []string          `json:"shell,omitempty"`
	User       string            `json:"user,omitempty"`
	WorkDir    string            `json:"workdir,omitempty"`
}

var configSchema schematypes.Schema = schematypes.Object{
	Title: "Configuration for qemu-guest-tools",
	Description: util.Markdown(`
			Configuration for 'taskcluster-worker qemu-guest-tools', this configures
			which user to run tasks under, what environment variables to set and
			which entrypoint to execute, if any. It also specifies which default shell
			to use when system-default shell is requested.
	`),
	Properties: schematypes.Properties{
		"entrypoint": schematypes.Array{
			Description: util.Markdown(`
				Command wrapper for the task command.

				If 'task.payload.command = [a, b]' and 'entrypoint = [wrapper]', then
				command executed for the task will be: 'wrapper a b'.
				This similar to 'ENTRYPOINT' in 'dockerfile'.
			`),
			Items: schematypes.String{},
		},
		"env": schematypes.Map{
			Description: util.Markdown(`
				Both task commands and interactive shells will inherit environment
				variables given to 'taskcluster-worker qemu-guest-tools', this option
				allows for additional environment variables. These can be overwritten
				by the task definition.
			`),
			Values: schematypes.String{},
		},
		"shell": schematypes.Array{
			Description: util.Markdown(`
				Default system shell.
			`),
			Items: schematypes.String{},
		},
		"user": schematypes.String{
			Description: util.Markdown(`
				User to run commands and interactive shells under, defaults to whatever
				user the guest-tools are running under.
			`),
		},
		"workdir": schematypes.String{
			Description: util.Markdown(`
				Working directory to run commands and interactive shells under, defaults
				to whatever working directory the guest-tools are running under.
			`),
		},
	},
}
