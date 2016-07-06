package osxnative

import "github.com/taskcluster/taskcluster-worker/engines/extpoints"

func init() {
	// Register the mac engine as an import side-effect
	extpoints.EngineProviders.Register(engineProvider{}, "macosx")
}
