package nativeengine

import (
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

type config struct {
	Groups     []string `json:"groups,omitempty"`
	CreateUser bool     `json:"createUser"`
}

var configSchema = schematypes.Object{
	MetaData: schematypes.MetaData{
		Title: "Native Engine Config",
		Description: util.Markdown(`
			Configuration for the native engine, this engines creates
			a system user-account per task, and deletes user-account when task
			is completed.
		`),
	},
	Properties: schematypes.Properties{
		"groups": schematypes.Array{
			MetaData: schematypes.MetaData{
				Title: "Group Memberships",
				Description: util.Markdown(`
					List of system user-groups that the temporary
					task-users should be be granted membership of.
				`),
			},
			Items: schematypes.String{
				MetaData: schematypes.MetaData{
					Title:       "Group Name",
					Description: "Name of a user-group that task-users should be assigned",
				},
				Pattern: "^[a-zA-Z0-9_.-]+$",
			},
		},
		"createUser": schematypes.Boolean{
			MetaData: schematypes.MetaData{
				Title: "Create User per Task",
				Description: util.Markdown(`
					Tells if a system user should be created to run a command.

					When set to 'true', a new user is created on-the-fly to run each task.
					It runs the command from within the user's home directory.
					If 'false', the command runs without changing userid, hence, tasks
					will run with the same user as the worker does.
				`),
			},
		},
	},
	Required: []string{
		"createUser",
	},
}
