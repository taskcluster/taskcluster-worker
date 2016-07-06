package qemuengine

import "github.com/taskcluster/taskcluster-worker/engines/extpoints"

func init() {
	extpoints.EngineProviders.Register(engineProvider{}, "qemu")
}
