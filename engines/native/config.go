package nativeengine

import schematypes "github.com/taskcluster/go-schematypes"

type config struct {
	Groups []string `json:"groups,omitempty"`
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
	},
	Required: []string{},
}
