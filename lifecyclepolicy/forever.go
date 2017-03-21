package lifecyclepolicy

import (
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type foreverProvider struct{}

func (foreverProvider) ConfigSchema() schematypes.Object {
	return schematypes.Object{
		MetaData: schematypes.MetaData{
			Title:       "Forever Life-Cycle Policy",
			Description: "A forever life-cycle policy does nothing to end the life-cycle of the worker.",
		},
		Properties: schematypes.Properties{
			"stopOnError": schematypes.Boolean{
				MetaData: schematypes.MetaData{
					Title:       "Stop On Non-Fatal Errors",
					Description: "If true, this policy will gracefully stop on non-fatal errors.",
				},
			},
		},
	}
}

func (foreverProvider) NewLifeCyclePolicy(options Options) LifeCyclePolicy {
	var c struct {
		StopError bool `json:"stopOnError"`
	}
	schematypes.MustValidateAndMap(foreverProvider{}.ConfigSchema(), options.Config, &c)
	return &ForeverLifeCyclePolicy{
		Stoppable: options.Worker,
		StopError: c.StopError,
	}
}

// A ForeverLifeCyclePolicy never stops the worker.
type ForeverLifeCyclePolicy struct {
	Base
	runtime.Stoppable
	StopError bool
}

// ReportNonFatalError will gracefully stop the worker if StopError is true
func (p *ForeverLifeCyclePolicy) ReportNonFatalError() {
	if p.StopError && p.Stoppable != nil {
		p.StopGracefully()
	}
}

func init() {
	Register("forever", foreverProvider{})
}
