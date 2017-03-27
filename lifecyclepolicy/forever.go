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
		StopOnError bool `json:"stopOnError"`
	}
	schematypes.MustValidateAndMap(foreverProvider{}.ConfigSchema(), options.Config, &c)
	return &ForeverLifeCyclePolicy{
		StopOnError: c.StopOnError,
	}
}

// A ForeverLifeCyclePolicy never stops the worker.
type ForeverLifeCyclePolicy struct {
	StopOnError bool
}

// NewController returns a Controller implemeting the ForeverLifeCyclePolicy
func (p *ForeverLifeCyclePolicy) NewController(worker runtime.Stoppable) Controller {
	return &foreverController{
		Stoppable:   worker,
		StopOnError: p.StopOnError,
	}
}

type foreverController struct {
	Base
	runtime.Stoppable
	StopOnError bool
}

func (c *foreverController) ReportNonFatalError() {
	if c.StopOnError {
		c.StopGracefully()
	}
}

func init() {
	Register("forever", foreverProvider{})
}
