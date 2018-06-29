package relengapi

import (
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

type config struct {
	Domain string `json:"domain"`
	Token string `json:"token"`
}

var configSchema = schematypes.Object{
	Title: "Releng API proxy Plugin",
	Description: util.Markdown(`
	The relengapi proxy provide access to the
	[Releng API](https://wiki.mozilla.org/ReleaseEngineering/Applications/RelengAPI).
	`),
	Properties: schematypes.Properties{
		"domain": schematypes.String{
			Title: "Releng API base domain",
			Description: util.Markdown(`
			The releng API base domain. Default: mozilla-releng.net.
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
