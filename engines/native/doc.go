// Package nativeengine provides an engine with minimal sandboxing relying on
// per-task user accounts, temporary folders and process isolation.
//
// Platform specific methods such as run sub-process under a difference user,
// add/remove users and management of user permissions are all implemented in
// the system/ sub-package.
package nativeengine

import "github.com/taskcluster/taskcluster-worker/runtime"

var debug = runtime.Debug("native")
