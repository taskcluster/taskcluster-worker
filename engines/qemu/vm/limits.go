package vm

import (
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

// MachineLimits imposes limits on a virtual machine definition.
type MachineLimits struct {
	MaxMemory      int `json:"maxMemory"`
	MaxCPUs        int `json:"maxCPUs"`
	DefaultThreads int `json:"defaultThreads"`
}

// MachineLimitsSchema is the schema for MachineOptions.
var MachineLimitsSchema = schematypes.Object{
	Title: "Machine Limits",
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
		"maxCPUs": schematypes.Integer{
			Title: "Max CPUs",
			Description: util.Markdown(`
				Maximum number of CPUs a virtual machine can use.

				This is the product of 'threads', 'cores' and 'sockets' as specified
				in the machine definition 'machine.json'. If the virtual machine image
				does not specify CPU requires it will be given
        'maxCPUs / defaultThreads' number of cores in a single socket.
			`),
			Minimum: 1,
			Maximum: 255, // Maximum allowed by QEMU
		},
		"defaultThreads": schematypes.Integer{
			Title: "Default CPU Threads",
			Description: util.Markdown(`
				Number of CPU threads to assign, if assigning default values for
				'threads', 'cores' and 'sockets' based on 'maxCPUs'.

				This should generally default to 2, if the host features hyperthreading,
				otherwise 1 is likely ideal.
			`),
			Minimum: 1,
			Maximum: 255,
		},
	},
	Required: []string{
		"maxMemory",
		"maxCPUs",
		"defaultThreads",
	},
}
