package tcproxy

import (
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

var payloadSchema = schematypes.Object{
	Properties: schematypes.Properties{
		"disableTaskclusterProxy": schematypes.Boolean{
			Title: "Disable Proxy",
			Description: util.Markdown(`
				The taskcluster proxy forwards requests to the proxy 'tcproxy'
				while attaching a request signature covering 'task.scopes'. The proxy
				is enabled by default, this option can be used to disable it per-task.

				Please refer to engine specific documentation for how to access the
				proxy, often it is something like: 'http://<hostname>/<proxy>/<...>',
				hence, forwarding to the queue would be
				'http://<hostname>/tcproxy/queue.taskcluster.net/...'.
			`),
		},
	},
}

type payload struct {
	DisableTaskclusterProxy bool `json:"disableTaskclusterProxy"`
}
