package qemuengine

import "github.com/taskcluster/taskcluster-worker/engines"

func init() {
	engines.RegisterEngine("qemu", engineProvider{})
}
