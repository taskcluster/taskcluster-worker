// Package tcproxy provides a taskcluster-worker plugin that exposes a proxy
// that signs requests with taskcluster credentials matching task.scopes.
//
// Hence, if task-specific code running in a sandbox sends a request through
// this proxy the request will be signed with task.scopes, enabling
// task-specific code to make authenticated requests without obtaining
// credentials that could be leaked.
package tcproxy

import "github.com/taskcluster/taskcluster-worker/runtime/util"

var debug = util.Debug("tcproxy")
