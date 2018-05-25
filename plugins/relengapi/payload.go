package relengapi

import (
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

var payloadSchema = schematypes.Object{
	Properties: schematypes.Properties{
		"enableRelengAPIProxy": schematypes.Boolean{
			Title: "Enable Proxy",
			Description: util.Markdown(`
				The relengapi proxy forwards requests to the proxy 'relengapi'. The proxy
				is disabled by default, this option can be used to enable it per-task.

				Please refer to engine specific documentation for how to access the
				proxy, often it is something like: 'http://<hostname>/<proxy>/<...>',
				hence, forwarding to the queue would be
				'http://<hostname>/relengapi/<request>/...'.
      `),
		},
	},
}

type payload struct {
	EnableRelengAPIProxy bool `json:"enableRelengAPIProxy"`
}
