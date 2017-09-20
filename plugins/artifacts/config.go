package artifacts

import (
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

type config struct {
	PrivateKey         string `json:"privateKey"`
	AlwaysCreateCOT    bool   `json:"alwaysCreateCOT"`
	CreateCertifiedLog string `json:"createCertifiedLog"`
}

const (
	optionAlways   = "always"
	optionOptional = "optional"
	optionNever    = "never"
)

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
		"alwaysCreateCOT": schematypes.Boolean{
			Title: "Always Create COT Certificate",
			Description: util.Markdown(`
				If set to 'true', all tasks will receive a chain-of-trust certificate.

				Otherwise, chain-of-trust certicates is a feature that can be enabled on
				a per-task level, using a setting in 'task.payload'.
			`),
		},
		"createCertifiedLog": schematypes.StringEnum{
			Title: "Create certified task log",
			Description: util.Markdown(`
				Whether to create 'public/logs/certified.log' before generating
				chain-of-trust artifact. This will be a partial task log that is signed
				in the COT certificate.

				Defaults to 'never' if not given.
			`),
			Options: []string{
				optionAlways,
				optionOptional,
				optionNever,
			},
		},
	},
}
