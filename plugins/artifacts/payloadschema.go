package artifacts

import (
	"time"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

type payload struct {
	Artifacts    []artifact `json:"artifacts"`
	CreateCOT    bool       `json:"chainOfTrust"`
	CertifiedLog bool       `json:"certifiedLog"`
}

type artifact struct {
	Type    string    `json:"type"`
	Path    string    `json:"path"`
	Name    string    `json:"name"`
	Expires time.Time `json:"expires"`
}

const (
	typeFile      = "file"
	typeDirectory = "directory"
)

var artifactSchema = schematypes.Array{
	Title:       "Artifacts",
	Description: "Artifacts to be published",
	Items: schematypes.Object{
		Properties: schematypes.Properties{
			"type": schematypes.StringEnum{
				Title: "Upload type",
				Description: util.Markdown(`
					Artifacts can be either an individual 'file' or a 'directory'
					containing potentially multiple files with recursively included
					subdirectories
				`),
				Options: []string{typeFile, typeDirectory},
			},
			"path": schematypes.String{
				Title:       "Artifact Path",
				Description: "File system path of artifact",
				Pattern:     `^.*[^/]$`,
			},
			"name": schematypes.String{
				Title: "Artifact Name",
				Description: util.Markdown(`
					This will be the leading path to directories and the full name
					for files that are uploaded to s3. It must not begin or end
					with '/' and must only contain printable ascii characters
					otherwise.
				`),
				Pattern: `^([\x20-\x2e\x30-\x7e][\x20-\x7e]*)[\x20-\x2e\x30-\x7e]$`,
			},
			"expires": schematypes.DateTime{
				Title:       "Expiration Date",
				Description: "",
			},
		},
		Required: []string{"type", "path", "name"},
	},
}
