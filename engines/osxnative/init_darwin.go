package osxnative

import "github.com/taskcluster/taskcluster-worker/engines"

func init() {
	// Register the mac engine as an import side-effect
	engines.RegisterEngine("macosx", engineProvider{})
}
