package relengapi

import (
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

type config struct {
	Host  string `json:"host"`
	Token string `json:"token"`
}

var configSchema = schematypes.Object{
	Title: "Releng API proxy Plugin",
	Description: util.Markdown(`
	The relengapi proxy provide access to the
	[Releng API](https://wiki.mozilla.org/ReleaseEngineering/Applications/RelengAPI).
	`),
	Properties: schematypes.Properties{
		"host": schematypes.URI{
			Title: "Releng API host endpoint",
			Description: util.Markdown(`
			The releng API host endpoint. Default: https://api.pub.build.mozilla.org/.
			`),
		},
		"token": schematypes.String{
			Title: "Releng API token",
			Description: util.Markdown(`
			The issue token to use. This token is used to retrieve a temporary token
			that is used to make requests to the endpoint.
			`),
		},
	},
}
