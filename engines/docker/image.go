package dockerengine

import (
	schematypes "github.com/taskcluster/go-schematypes"
)

type imageType struct {
	Repository string  `json:"repository"`
	Tag        string  `json:"tag"`
	engine     *engine `json:"-"`
}

var imageSchema = schematypes.Object{
	Title:       "Image",
	Description: "Image to use for task.",
	Properties: schematypes.Properties{
		"repository": schematypes.String{
			Title: "Repository",
			Description: "Repository from which image must be \b" +
				"pulled.",
		},
		"tag": schematypes.String{
			Title:       "Tag",
			Description: "Image tag to pull from repository.",
		},
	},
	Required: []string{"tag"},
}
