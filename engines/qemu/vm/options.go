package vm

import (
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

// MachineOptions specifies the limits on the virtual machine limits, default
// values and options.
type MachineOptions struct {
	MaxMemory int `json:"maxMemory"`
	MaxCores  int `json:"maxCores"`
}

// MachineOptionsSchema is the schema for MachineOptions.
var MachineOptionsSchema = schematypes.Object{
	Title: "Machine Options",
	Description: util.Markdown(`
		Limitations and default values for the virtual machine
		configuration. Tasks with machine images that does not satisfy
		these limitations will be resolved 'malformed-payload'.
	`),
	Properties: schematypes.Properties{
		"maxMemory": schematypes.Integer{
			Title: "Max Memory",
			Description: util.Markdown(`
				Maximum allowed virtual machine memory in MiB. This is
				also the default memory if the machine image does not
				specify memory requirements. If the machine specifies a
				memory higher than this, the task will fail to run.
			`),
			Minimum: 0,
			Maximum: 1024 * 1024, // 1 TiB
		},
		"maxCores": schematypes.Integer{
			Title: "Max CPU Cores",
			Description: util.Markdown(`
				Maximum number of CPUs a virtual machine can use.

				This is the product of 'threads', 'cores' and 'sockets' as specified
				in the machine definition 'machine.json'. If the virtual machine image
				does not specify CPU requires it will be given 'maxCores' number of
				cores in a single socket.
			`),
			Minimum: 1,
			Maximum: 255, // Maximum allowed by QEMU
		},
	},
	Required: []string{
		"maxMemory",
		"maxCores",
	},
}
