package artifacts

import (
	"time"

	schematypes "github.com/taskcluster/go-schematypes"
)

type payloadType struct {
	Artifacts []artifact `json:"artifacts"`
}
type artifact struct {
	Type    string    `json:"type"`
	Path    string    `json:"path"`
	Name    string    `json:"name"`
	Expires time.Time `json:"expires"`
}

var payloadSchema = schematypes.Object{
	Properties: schematypes.Properties{
		"artifacts": schematypes.Array{
			MetaData: schematypes.MetaData{
				Title:       "Artifacts",
				Description: "Artifacts to be published",
			},
			Items: schematypes.Object{
				Properties: schematypes.Properties{
					"type": schematypes.StringEnum{
						MetaData: schematypes.MetaData{
							Title: "Upload type",
							Description: "Artifacts can be either an individual `file` or " +
								"a `directory` containing potentially multiple files with " +
								"recursively included subdirectories",
						},
						Options: []string{"file", "directory"},
					},
					"path": schematypes.String{
						MetaData: schematypes.MetaData{
							Title:       "Artifact Path",
							Description: "File system path of artifact",
						},
						Pattern: `^.*[^/]$`,
					},
					"name": schematypes.String{
						MetaData: schematypes.MetaData{
							Title: "Artifact Name",
							Description: "" +
								"This will be the leading path to directories and the full name\n" +
								"for files that are uploaded to s3. It must not begin or end\n" +
								"with '/' and must only contain printable ascii characters\n" +
								"otherwise.",
						},
						Pattern: `^([\x20-\x2e\x30-\x7e][\x20-\x7e]*)[\x20-\x2e\x30-\x7e]$`,
					},
					"expires": schematypes.DateTime{
						MetaData: schematypes.MetaData{
							Title:       "Expiration Date",
							Description: "",
						},
					},
				},
				Required: []string{"type", "path", "name"},
			},
		},
	},
}
