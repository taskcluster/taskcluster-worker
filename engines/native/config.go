package nativeengine

import schematypes "github.com/taskcluster/go-schematypes"

type config struct {
	Groups     []string `json:"groups,omitempty"`
	CreateUser bool     `json:"createUser,omitempty"`
}

var configSchema = schematypes.Object{
	MetaData: schematypes.MetaData{
		Title: "Native Engine Config",
		Description: "Configuration for the native engine, this engines creates " +
			"a system user-account per task, and deletes user-account when task " +
			"is completed.",
	},
	Properties: schematypes.Properties{
		"groups": schematypes.Array{
			MetaData: schematypes.MetaData{
				Title: "Group Memberships",
				Description: "List of system user-groups that the temporary " +
					"task-users should be be granted membership of.",
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
				Title: "Tells if a user should be created to run a command",
				Description: `When set to true, a new user is created on the fly to run
				the command. It runs the command from whitin the user's home directory.
				If false, the command runs without changing userid`,
			},
		},
	},
	Required: []string{},
}
