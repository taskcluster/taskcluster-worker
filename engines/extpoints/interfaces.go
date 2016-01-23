// Package extpoints provides methods that engine plugins can register their
// implements with as an import side-effect.
package extpoints

import (
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type EngineProvider func(environment *runtime.Environment, options *engines.EngineOptions) (engines.Engine, error)
