package nativetest

import (
	_ "github.com/taskcluster/taskcluster-worker/engines/native"
	_ "github.com/taskcluster/taskcluster-worker/plugins/artifacts"
	_ "github.com/taskcluster/taskcluster-worker/plugins/env"
	_ "github.com/taskcluster/taskcluster-worker/plugins/livelog"
	_ "github.com/taskcluster/taskcluster-worker/plugins/maxruntime"
	_ "github.com/taskcluster/taskcluster-worker/plugins/reboot"
	_ "github.com/taskcluster/taskcluster-worker/plugins/stoponerror"
	_ "github.com/taskcluster/taskcluster-worker/plugins/success"
	_ "github.com/taskcluster/taskcluster-worker/plugins/watchdog"
)
