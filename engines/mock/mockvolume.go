package mockengine

import "github.com/taskcluster/taskcluster-worker/engines"

// A mock volume basically hold a bit value that can be set or cleared
type volume struct {
	engines.VolumeBase
	value bool
}
