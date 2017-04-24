package vm

import (
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

// MachineOptions specifies the limits on the virtual machine limits, default
// values and options.
type MachineOptions struct {
	MaxMemory int `json:"maxMemory"`
}

// MachineOptionsSchema is the schema for MachineOptions.
var MachineOptionsSchema = schematypes.Object{
	Title: "Machine Options",
	Description: util.Markdown(`
		Limitations and default values for the virtual machine
		configuration. Tasks with machine images that doesn't satisfy
		these limitations will be resolved 'malformed-payload'.
	`),
	Properties: schematypes.Properties{
		"maxMemory": schematypes.Integer{
			Title: "Max Memory",
			Description: util.Markdown(`
				Maximum allowed virtual machine memory in MiB. This is
				also the default memory if the machine image doesn't
				specify memory requirements. If the machine specifies a
				memory higher than this, the task will fail to run.
			`),
			Minimum: 0,
			Maximum: 1024 * 1024, // 1 TiB
		},
	},
	Required: []string{
		"maxMemory",
	},
}
