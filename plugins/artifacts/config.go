package artifacts

import (
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

type config struct {
	PrivateKey string `json:"privateKey"`
}

var configSchema = schematypes.Object{
	Title: "Artifact Configuration",
	Description: util.Markdown(`
		Configuration for artifact plugin. This is mostly COT (chain-of-trust)
		configuration, such as private key.
	`),
	Properties: schematypes.Properties{
		"privateKey": schematypes.String{
			Title: "COT Private Key",
			Description: util.Markdown(`
				GPG armoured private key (unencrypted) for signing chain-of-trust
				certificates.

				If not given, chain-of-trust signing will be disabled.
			`),
		},
	},
}
